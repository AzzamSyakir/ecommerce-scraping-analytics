package controllers

import (
	"context"
	"ecommerce-scraping-analytics/internal/config"
	"ecommerce-scraping-analytics/internal/entity"
	"ecommerce-scraping-analytics/internal/rabbitmq/producer"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

type ScrapingController struct {
	Producer *producer.ScrapingControllerProducer
	Rabbitmq *config.RabbitMqConfig
}

func NewScrapingController(rabbitMq *config.RabbitMqConfig, producer *producer.ScrapingControllerProducer) *ScrapingController {
	scrapingController := &ScrapingController{
		Producer: producer,
		Rabbitmq: rabbitMq,
	}
	return scrapingController
}

func (scrapingcontroller *ScrapingController) ScrapeSellerProduct(seller string) {
	// Extract functions for category and product details
	extractCategoryID := func(url string) string {
		re := regexp.MustCompile(`/c/([0-9]+)`)
		matches := re.FindStringSubmatch(url)
		if len(matches) > 1 {
			return matches[1]
		}
		return ""
	}
	extractProductID := func(url string) string {
		re := regexp.MustCompile(`/p/([0-9]+)`)
		matches := re.FindStringSubmatch(url)
		if len(matches) > 1 {
			return matches[1]
		}
		return ""
	}
	extractCategoryName := func(url string) string {
		re := regexp.MustCompile(`/c/[^/]+/([^/]+)`)
		matches := re.FindStringSubmatch(url)
		if len(matches) > 1 {
			return matches[1]
		}
		return ""
	}

	// Headers for scraping request
	headers := map[string]interface{}{
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
		"Accept-Encoding":           "gzip, deflate, br",
		"Accept-Language":           "en-US,en;q=0.9",
		"Cache-Control":             "max-age=0",
		"Connection":                "keep-alive",
		"Upgrade-Insecure-Requests": "1",
		"User-Agent":                "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.6778.69 Safari/537.36",
	}

	// Initialize Chromedp context
	ctx, cancel := chromedp.NewExecAllocator(
		context.Background(),
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.DisableGPU,
			chromedp.NoSandbox,
			chromedp.Flag("disable-plugins", true),
			chromedp.Flag("disable-background-timer-throttling", true),
			chromedp.Flag("disable-extensions", true),
			chromedp.Flag("blink-settings", "imagesEnabled=false"),
			chromedp.Flag("disable-features", "NetworkService,OutOfBlinkCors"),
			chromedp.Flag("headless", true),
		)...,
	)

	defer cancel()

	// Create a browser context from the allocator
	browserCtx, cancelBrowser := chromedp.NewContext(ctx)
	defer cancelBrowser()
	// Channel and pool definitions
	maxWorker := 20
	categoryCh := make(chan string)
	productCh := make(chan entity.Product)
	done := make(chan bool)
	categoryPool := make(chan struct{}, maxWorker)
	productListpool := make(chan struct{}, maxWorker)
	productDetailPool := make(chan struct{}, maxWorker)
	retryCategoryCh := make(chan string, maxWorker)
	retryProductCh := make(chan string, maxWorker)
	retryProductDetailCh := make(chan string, maxWorker)
	retryPool := make(chan struct{}, maxWorker)
	errCh := make(chan error, 1)

	// handling error
	go func() {
		for err := range errCh {
			messageError := "responseError"
			messageCombine := messageError + ": " + err.Error()
			scrapingcontroller.Producer.PublishScrapingData(messageCombine, scrapingcontroller.Rabbitmq.Channel, nil)
		}
	}()

	//Scrape Categories
	var categoryURLs []string
	go func() {
		var retryWg sync.WaitGroup
		defer func() {
			close(categoryCh)
			close(categoryPool)
		}()
		categoryPool <- struct{}{}
		defer func() { <-categoryPool }()
		baseUrl := "http://www.%s.ecrater.com%s"
		sellerCategory := fmt.Sprintf(baseUrl, seller, "/category.php")
		err := chromedp.Run(browserCtx,
			network.SetCacheDisabled(false),
			network.SetBlockedURLS([]string{"*.png", "*.jpg", "*.jpeg", "*.gif", "*.css", "*.js"}),
			network.Enable(),
			network.SetExtraHTTPHeaders(network.Headers(headers)),
			chromedp.Navigate(sellerCategory),
			chromedp.ActionFunc(func(ctx context.Context) error {
				checkService := `
				(() => {
					const h1Element = document.getElementsByTagName('h1')[0];
					return h1Element ? h1Element.textContent.trim() : '';
				})();
			`

				var pageTitle string
				if err := chromedp.Evaluate(checkService, &pageTitle).Do(ctx); err != nil {
					return err
				}

				if pageTitle == "Service Temporarily Unavailable" {
					retryCategoryCh <- sellerCategory
					return nil
				}
				js := `
				(() => {
					const links = document.querySelectorAll('section.clearfix a[href]');
					return [...links].map(link => link.href);
				})()
			`
				return chromedp.Evaluate(js, &categoryURLs).Do(ctx)
			}),
		)
		if err != nil {
			errCh <- err
			return
		}
		for _, url := range categoryURLs {
			categoryCh <- url
		}
		// Goroutine for retry categories Error
		go func() {
			for productUrl := range retryProductCh {
				retryWg.Add(1)
				go func(url string) {
					defer retryWg.Done()
					retryPool <- struct{}{}
					defer func() { <-retryPool }()
					retryErrProductCtx, cancel := chromedp.NewContext(browserCtx)
					defer cancel()
					err := chromedp.Run(retryErrProductCtx,
						network.SetCacheDisabled(false),
						network.SetBlockedURLS([]string{"*.png", "*.jpg", "*.jpeg", "*.gif", "*.css", "*.js"}),
						network.Enable(),
						network.SetExtraHTTPHeaders(network.Headers(headers)),
						chromedp.Navigate(url),
						chromedp.ActionFunc(func(ctx context.Context) error {
							js := `
							(() => {
								const links = document.querySelectorAll('section.clearfix a[href]');
								return [...links].map(link => link.href);
							})()
						`
							err := chromedp.Evaluate(js, &categoryURLs).Do(ctx)
							if err != nil {
								return err
							}
							for _, url := range categoryURLs {
								categoryCh <- url
							}
							return nil
						}),
					)
					if err != nil {
						errCh <- err
						return
					}
				}(productUrl)
			}
		}()
		retryWg.Wait()
	}()
	//Scrape Products from Categories
	go func() {
		var (
			wg                     sync.WaitGroup
			retryWg                sync.WaitGroup
			categoryProductResults []struct {
				Href string `json:"href"`
			}
		)
		defer func() {
			wg.Wait()
			close(productCh)
			close(productListpool)
		}()

		for url := range categoryCh {
			wg.Add(1)
			go func(url string) {
				defer wg.Done()
				productListpool <- struct{}{}
				defer func() { <-productListpool }()

				productCategoriesCtx, cancel := chromedp.NewContext(browserCtx)
				defer cancel()

				err := chromedp.Run(productCategoriesCtx,
					network.SetCacheDisabled(false),
					network.SetBlockedURLS([]string{"*.png", "*.jpg", "*.jpeg", "*.gif", "*.css", "*.js"}),
					network.Enable(),
					network.SetExtraHTTPHeaders(network.Headers(headers)),
					chromedp.Navigate(url),
					chromedp.WaitReady("body"),
					chromedp.ActionFunc(func(ctx context.Context) error {
						checkService := `
						(() => {
							const h1Element = document.getElementsByTagName('h1')[0];
							return h1Element ? h1Element.textContent.trim() : '';
						})();
					`

						var pageTitle string
						if err := chromedp.Evaluate(checkService, &pageTitle).Do(ctx); err != nil {
							return err
						}

						if pageTitle == "Service Temporarily Unavailable" {
							retryProductCh <- url
							return nil
						}
						js := `
						(() => {
							return [...document.querySelectorAll('#product-list-grid > li > div.product-details > h2 > a')]
								.map(a => ({ href: a.href }));
						})()
					`
						return chromedp.Evaluate(js, &categoryProductResults).Do(ctx)
					}),
				)
				if err != nil {
					errCh <- err
					return
				}
				for _, productResult := range categoryProductResults {
					productCh <- entity.Product{
						ProductID:  extractProductID(productResult.Href),
						ProductURL: productResult.Href,
					}
				}
			}(url)
		}
		// Goroutine for retry product from categories
		go func() {
			for productUrl := range retryProductCh {
				retryWg.Add(1)
				go func(url string) {
					defer retryWg.Done()
					retryPool <- struct{}{}
					defer func() { <-retryPool }()
					retryErrProductCtx, cancel := chromedp.NewContext(browserCtx)
					defer cancel()
					err := chromedp.Run(retryErrProductCtx,
						network.SetCacheDisabled(false),
						network.SetBlockedURLS([]string{"*.png", "*.jpg", "*.jpeg", "*.gif", "*.css", "*.js"}),
						network.Enable(),
						network.SetExtraHTTPHeaders(network.Headers(headers)),
						chromedp.Navigate(url),
						chromedp.ActionFunc(func(ctx context.Context) error {
							js := `
								(() => {
									return [...document.querySelectorAll('#product-list-grid > li > div.product-details > h2 > a')]
										.map(a => ({ href: a.href }));
								})()
							`
							err := chromedp.Evaluate(js, &categoryProductResults).Do(ctx)
							if err != nil {
								return err
							}
							for _, productResult := range categoryProductResults {
								productCh <- entity.Product{
									ProductID:  extractProductID(productResult.Href),
									ProductURL: productResult.Href,
								}
							}
							return nil
						}),
					)
					if err != nil {
						errCh <- err
						return
					}
				}(productUrl)
			}
		}()
		retryWg.Wait()
	}()
	// Scrape Product Details
	go func() {
		var (
			wg                  sync.WaitGroup
			retryWg             sync.WaitGroup
			mu                  sync.Mutex
			allCategoryProducts []entity.CategoryProducts
			products            []entity.Product
		)
		// scrape page productDetail
		for product := range productCh {
			wg.Add(1)
			go func(product entity.Product) {
				defer wg.Done()
				productDetailPool <- struct{}{}
				defer func() { <-productDetailPool }()

				productDetailCtx, cancel := chromedp.NewContext(browserCtx)
				defer cancel()

				productUrl := product.ProductURL
				if strings.Contains(product.ProductURL, ".ecrater.com") {
					pos := strings.Index(product.ProductURL, ".ecrater.com")
					if pos != -1 {
						productUrl = "https://www.ecrater.com" + product.ProductURL[pos+len(".ecrater.com"):]
					}
				}

				var productDetailsResults struct {
					Title     string `json:"title"`
					Available string `json:"available"`
					Sold      string `json:"sold"`
					Price     string `json:"price"`
				}

				// Scraping page productDetail
				err := chromedp.Run(productDetailCtx,
					network.SetBlockedURLS([]string{"*.png", "*.jpg", "*.jpeg", "*.gif", "*.css", "*.js"}),
					network.SetCacheDisabled(false),
					network.Enable(),
					network.SetExtraHTTPHeaders(network.Headers(headers)),
					chromedp.Navigate(productUrl),
					chromedp.WaitVisible("body", chromedp.ByQuery),
					chromedp.ActionFunc(func(ctx context.Context) error {
						checkService := `
						(() => {
							const h1Element = document.getElementsByTagName('h1')[0];
							return h1Element ? h1Element.textContent.trim() : '';
						})();
					`

						var pageTitle string
						if err := chromedp.Evaluate(checkService, &pageTitle).Do(ctx); err != nil {
							return err
						}

						if pageTitle == "Service Temporarily Unavailable" {
							retryProductDetailCh <- productUrl
							return nil
						}
						js := `
						(() => {
							const detailsElement = document.querySelector('#product-quantity');
							const priceRaw = document.querySelector('#price')?.textContent.trim() || '';
							const title = document.querySelector('#product-title > h1')?.textContent.trim() || '';
							const available = detailsElement?.textContent.split(',')[0]?.trim() || '';
							const sold = detailsElement?.querySelector('b')?.textContent.trim() || '0';
							return { 
								title, 
								available, 
								sold, 
								price: priceRaw ? '$' + priceRaw : '' 
							};
						})()
					`
						return chromedp.Evaluate(js, &productDetailsResults).Do(ctx)
					}),
				)
				if err != nil {
					errCh <- err
					return
				}

				mu.Lock()
				products = append(products, entity.Product{
					ProductID:    extractProductID(productUrl),
					ProductTitle: productDetailsResults.Title,
					ProductURL:   productUrl,
					ProductStock: productDetailsResults.Available,
					ProductPrice: productDetailsResults.Price,
					ProductSold:  productDetailsResults.Sold,
				})
				mu.Unlock()
			}(product)
		}

		// Goroutine for retry productDetail
		go func() {
			for productUrl := range retryProductDetailCh {
				retryWg.Add(1)
				go func(url string) {
					defer retryWg.Done()
					retryPool <- struct{}{}
					defer func() { <-retryPool }()

					var productDetailsResults struct {
						Title     string `json:"title"`
						Available string `json:"available"`
						Sold      string `json:"sold"`
						Price     string `json:"price"`
					}
					retryErrProductCtx, cancel := chromedp.NewContext(browserCtx)
					defer cancel()
					err := chromedp.Run(retryErrProductCtx,
						network.SetCacheDisabled(false),
						network.SetBlockedURLS([]string{"*.png", "*.jpg", "*.jpeg", "*.gif", "*.css", "*.js"}),
						network.Enable(),
						network.SetExtraHTTPHeaders(network.Headers(headers)),
						chromedp.Navigate(url),
						chromedp.ActionFunc(func(ctx context.Context) error {
							js := `
							(() => {
								const detailsElement = document.querySelector('#product-quantity');
								const priceRaw = document.querySelector('#price')?.textContent.trim() || '';
								const title = document.querySelector('#product-title > h1')?.textContent.trim() || '';
								const available = detailsElement?.textContent.split(',')[0]?.trim() || '';
								const sold = detailsElement?.querySelector('b')?.textContent.trim() || '0';
								return { 
									title, 
									available, 
									sold, 
									price: priceRaw ? '$' + priceRaw : '' 
								};
							})()
						`
							err := chromedp.Evaluate(js, &productDetailsResults).Do(ctx)
							if err != nil {
								return err
							}

							mu.Lock()
							products = append(products, entity.Product{
								ProductID:    extractProductID(url),
								ProductTitle: productDetailsResults.Title,
								ProductURL:   url,
								ProductStock: productDetailsResults.Available,
								ProductPrice: productDetailsResults.Price,
								ProductSold:  productDetailsResults.Sold,
							})
							mu.Unlock()
							return nil
						}),
					)
					if err != nil {
						errCh <- err
						return
					}
				}(productUrl)
			}
		}()

		// wait for all goroutine
		wg.Wait()
		retryWg.Wait()
		// close ctx, channel and pool
		close(retryCategoryCh)
		close(retryProductCh)
		close(retryProductDetailCh)
		close(done)
		close(productDetailPool)
		close(errCh)
		cancelBrowser()

		// Collect and Send Final Data
		for _, url := range categoryURLs {
			allCategoryProducts = append(allCategoryProducts, entity.CategoryProducts{
				CategoryName: extractCategoryName(url),
				CategoryID:   extractCategoryID(url),
				Products:     products,
			})
		}

		message := "responseSuccess"
		scrapingcontroller.Producer.PublishScrapingData(message, scrapingcontroller.Rabbitmq.Channel, allCategoryProducts)
	}()

	<-done
}
