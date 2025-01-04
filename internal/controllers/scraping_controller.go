package controllers

import (
	"context"
	"ecommerce-scraping-analytics/internal/config"
	"ecommerce-scraping-analytics/internal/entity"
	"ecommerce-scraping-analytics/internal/rabbitmq/producer"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

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

func (scrapingcontroller *ScrapingController) ScrapeAllSellerProducts(seller string) {
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
			chromedp.Flag("headless", false),
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
		baseUrl := "http://%s.ecrater.com%s"
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
						productUrl = "https://ecrater.com" + product.ProductURL[pos+len(".ecrater.com"):]
					}
				}

				var productDetailsResults struct {
					Title     string `json:"title"`
					Available string `json:"available"`
					Sold      int    `json:"sold"`
					Price     string `json:"price"`
				}

				// Scraping page productDetail
				err := chromedp.Run(productDetailCtx,
					network.SetBlockedURLS([]string{"*.png", "*.jpg", "*.jpeg", "*.gif", "*.css", "*.js"}),
					network.SetCacheDisabled(false),
					network.Enable(),
					network.SetExtraHTTPHeaders(network.Headers(headers)),
					chromedp.Navigate(productUrl),
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
								const title = document.querySelector('#product-title > h1')?.firstChild?.nodeValue.trim() || '';
								const available = detailsElement?.textContent.split(',')[0]?.trim() || '';
								const sold = parseInt(detailsElement?.querySelector('b')?.textContent.trim() || '0', 10);
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
						Sold      int    `json:"sold"`
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
								const sold = parseInt(detailsElement?.querySelector('b')?.textContent.trim() || '0', 10);
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
func (scrapingcontroller *ScrapingController) ScrapeSoldSellerProducts(seller string) {
	extractProductID := func(url string) string {
		re := regexp.MustCompile(`/p/([0-9]+)`)
		matches := re.FindStringSubmatch(url)
		if len(matches) > 1 {
			return matches[1]
		}
		return ""
	}
	extractCategoryName := func(url string) string {
		cleanURL := strings.Split(url, "?")[0]

		re := regexp.MustCompile(`/c/[^/]+/([^/]+)`)
		matches := re.FindStringSubmatch(cleanURL)
		if len(matches) > 1 {
			return matches[1]
		}
		return ""
	}

	extractCategoryID := func(url string) string {
		cleanURL := strings.Split(url, "?")[0]

		re := regexp.MustCompile(`/c/([0-9]+)`)
		matches := re.FindStringSubmatch(cleanURL)
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
			chromedp.Flag("no-sandbox", true),
			// chromedp.Flag("headless", false),
		)...,
	)

	defer cancel()

	// Create a browser context from the allocator
	browserCtx, cancelBrowser := chromedp.NewContext(ctx)
	defer cancelBrowser()
	// Channel and pool definitions
	maxWorker := 20
	categoryCh := make(chan string)
	productCh := make(chan entity.ProductWithCategory)
	done := make(chan bool)
	categoryPool := make(chan struct{}, maxWorker)
	productListpool := make(chan struct{}, maxWorker)
	productDetailPool := make(chan struct{}, maxWorker)
	retryCategoryCh := make(chan string, maxWorker)
	retryProductCh := make(chan string, maxWorker)
	retryProductDetailCh := make(chan entity.ProductWithCategory, maxWorker)
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
		baseUrl := "http://%s.ecrater.com%s"
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
		// goroutine scrape product from categories
		for url := range categoryCh {
			categoryUrl := fmt.Sprintf("%s?&perpage=80", url)
			wg.Add(1)
			go func(url string) {
				defer wg.Done()
				productListpool <- struct{}{}
				defer func() { <-productListpool }()

				var nextPageHref string
				var currentPageResults []struct {
					Href string `json:"href"`
				}
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
						// Check if the page is temporarily unavailable
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

						// Scrape product links from the current page
						jsScrapeProduct := `
								(() => {
										return [...document.querySelectorAll('#product-list-grid > li')]
												.map(li => {
														const link = li.querySelector('div.product-details > h2 > a');
														return link ? { href: link.href, text: link.textContent.trim() } : '';
												})
												.filter(item => item !== '');
								})();
						`
						if err := chromedp.Evaluate(jsScrapeProduct, &currentPageResults).Do(ctx); err != nil {
							return err
						}
						categoryProductResults = append(categoryProductResults, currentPageResults...)
						// Check if the next page button exists and extract its href
						jsCheckNextPage := `
								(() => {
										const nextPageBtn = document.querySelector('#page-header-controls > ul > li > a');
										return nextPageBtn ? nextPageBtn.href : "";
								})();
						`
						if err := chromedp.Evaluate(jsCheckNextPage, &nextPageHref).Do(ctx); err != nil {
							return err
						}

						// If a next page is available, navigate to it and repeat scraping
						if nextPageHref != "" {
							nextPageHref = fmt.Sprintf("%s&perpage=80", nextPageHref)
							err := chromedp.Navigate(nextPageHref).Do(ctx)
							if err != nil {
								return err
							}

							// Wait for the product list grid to be visible
							if err := chromedp.WaitVisible("#product-list-grid", chromedp.ByID).Do(ctx); err != nil {
								return err
							}

							// Recursively scrape the next page
							return chromedp.ActionFunc(func(ctx context.Context) error {
								if err := chromedp.Evaluate(jsScrapeProduct, &currentPageResults).Do(ctx); err != nil {
									return err
								}
								categoryProductResults = append(categoryProductResults, currentPageResults...)
								return nil
							}).Do(ctx)
						}

						return nil
					}),
				)
				if err != nil {
					errCh <- err
					return
				}
				for _, productResult := range categoryProductResults {

					productCh <- entity.ProductWithCategory{
						CategoryURL: url,
						Product: entity.Product{
							ProductID:  extractProductID(productResult.Href),
							ProductURL: productResult.Href,
						},
					}
				}

			}(categoryUrl)
		}

		// Goroutine for retry scrape product from categories
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
							// Scrape product links from the category page
							jsScrapeProduct := `
								(() => {
										return [...document.querySelectorAll('#product-list-grid > li > div.product-details > h2 > a')]
												.map(a => ({ href: a.href }));
								})()
						`
							// Check if the next page button is available after scraping, if available click and move to the next page
							ctxWithTimeout, cancel := context.WithTimeout(ctx, 2*time.Second)
							defer cancel()

							chromedp.WaitVisible("#page-header-controls > ul > li > a", chromedp.ByQuery).Do(ctxWithTimeout)

							checkNextPageButton := `
								(() => {
										const nextPageBtn = document.querySelector('#page-header-controls > ul > li > a');
										return nextPageBtn ? true : false;
								})()
						`
							var hasNextPage bool
							if err := chromedp.Evaluate(checkNextPageButton, &hasNextPage).Do(ctx); err != nil {
								return err
							}

							if hasNextPage {
								err := chromedp.Evaluate(jsScrapeProduct, &categoryProductResults).Do(ctx)
								if err != nil {
									return err
								}
								err = chromedp.Click("#page-header-controls > ul > li > a").Do(ctx)
								if err != nil {
									return err
								}
								err = chromedp.WaitVisible("#product-list-grid", chromedp.ByID).Do(ctx)
								if err != nil {
									return err
								}
								return chromedp.ActionFunc(func(ctx context.Context) error {
									js := `
												(() => {
														return [...document.querySelectorAll('#product-list-grid > li > div.product-details > h2 > a')]
																.map(a => ({ href: a.href }));
												})()
										`
									return chromedp.Evaluate(js, &categoryProductResults).Do(ctx)
								}).Do(ctx)
							}
							return chromedp.Evaluate(jsScrapeProduct, &categoryProductResults).Do(ctx)
						}),
					)
					if err != nil {
						errCh <- err
						return
					}
					for _, productResult := range categoryProductResults {
						if url == "https://mahendraherbal.ecrater.com/c/2252218/my-ebay" {
							fmt.Println("productCategoryResult in retry scraping", productResult.Href)
						}
						productCh <- entity.ProductWithCategory{
							CategoryURL: url,
							Product: entity.Product{
								ProductID:  extractProductID(productResult.Href),
								ProductURL: productResult.Href,
							},
						}
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
			categoryProductsMap = make(map[string]map[string]entity.Product)
			categoryNamesMap    = make(map[string]string)
		)
		// scrape page productDetail
		for productCategory := range productCh {
			wg.Add(1)
			go func(productCategory entity.ProductWithCategory) {
				defer wg.Done()
				productDetailPool <- struct{}{}
				defer func() { <-productDetailPool }()

				productDetailCtx, cancel := chromedp.NewContext(browserCtx)
				defer cancel()

				productUrl := productCategory.Product.ProductURL
				if strings.Contains(productUrl, ".ecrater.com") {
					pos := strings.Index(productUrl, ".ecrater.com")
					if pos != -1 {
						productUrl = "https://www.ecrater.com" + productUrl[pos+len(".ecrater.com"):]
					}
				}

				var productDetailsResults struct {
					Title     string `json:"title"`
					Available string `json:"available"`
					Sold      int    `json:"sold"`
					Price     string `json:"price"`
				}

				err := chromedp.Run(productDetailCtx,
					network.SetBlockedURLS([]string{"*.png", "*.jpg", "*.jpeg", "*.gif", "*.css", "*.js"}),
					network.SetCacheDisabled(false),
					network.Enable(),
					network.SetExtraHTTPHeaders(network.Headers(headers)),
					chromedp.Navigate(productUrl),
					chromedp.ActionFunc(func(ctx context.Context) error {
						checkService := `
							(() => document.querySelector('h1')?.textContent.trim())()
						`
						var pageTitle string
						if err := chromedp.Evaluate(checkService, &pageTitle).Do(ctx); err != nil {
							return err
						}
						if pageTitle == "Service Temporarily Unavailable" {
							retryProductDetailCh <- entity.ProductWithCategory{
								CategoryURL: productCategory.CategoryURL,
								Product: entity.Product{
									ProductID:  extractProductID(productUrl),
									ProductURL: productUrl,
								},
							}
							return nil
						}

						js := `
							(() => {
									const details = document.querySelector('#product-quantity');
									const titleElement = document.querySelector('#product-title h1');
									const title = Array.from(titleElement.childNodes)
											.filter(node => node.nodeType === Node.TEXT_NODE)
											.map(node => node.textContent.trim())
											.join(' ')
											.replace(/\s+/g, ' ')
											.trim();
									const priceRaw = document.querySelector('#price')?.textContent.trim();
									const sold = parseInt(details?.querySelector('b')?.textContent.trim() || '0', 10);
									const available = details?.textContent.split(',')[0]?.trim();
									return { title, available, sold, price: priceRaw };
							})()
					`
						err := chromedp.Evaluate(js, &productDetailsResults).Do(ctx)
						if err != nil {
							return err
						}
						if productDetailsResults.Sold == 0 {
							return nil
						}
						mu.Lock()
						defer mu.Unlock()
						categoryUrl := productCategory.CategoryURL
						categoryID := extractCategoryID(categoryUrl)
						categoryName := extractCategoryName(categoryUrl)

						if _, exists := categoryNamesMap[categoryID]; !exists {
							categoryNamesMap[categoryID] = categoryName
						}

						if _, exists := categoryProductsMap[categoryID]; !exists {
							categoryProductsMap[categoryID] = make(map[string]entity.Product)
						}

						productID := extractProductID(productUrl)
						if _, exists := categoryProductsMap[categoryID][productID]; !exists {
							categoryProductsMap[categoryID][productID] = entity.Product{
								ProductID:    productID,
								ProductTitle: productDetailsResults.Title,
								ProductURL:   productUrl,
								ProductStock: productDetailsResults.Available,
								ProductPrice: productDetailsResults.Price,
								ProductSold:  productDetailsResults.Sold,
							}
						}
						return nil
					}),
				)
				if err != nil {
					errCh <- err
					return
				}
			}(productCategory)
		}
		// Goroutine for retry productDetail
		go func() {
			for productUrl := range retryProductDetailCh {
				retryWg.Add(1)
				go func(productCategory entity.ProductWithCategory) {
					defer retryWg.Done()
					retryPool <- struct{}{}
					defer func() { <-retryPool }()

					var productDetailsResults struct {
						Title     string `json:"title"`
						Available string `json:"available"`
						Sold      int    `json:"sold"`
						Price     string `json:"price"`
					}
					productUrl := productCategory.Product.ProductURL
					retryErrProductCtx, cancel := chromedp.NewContext(browserCtx)
					defer cancel()

					err := chromedp.Run(retryErrProductCtx,
						network.SetCacheDisabled(false),
						network.SetBlockedURLS([]string{"*.png", "*.jpg", "*.jpeg", "*.gif", "*.css", "*.js"}),
						network.Enable(),
						network.SetExtraHTTPHeaders(network.Headers(headers)),
						chromedp.Navigate(productUrl),
						chromedp.ActionFunc(func(ctx context.Context) error {
							js := `
							(() => {
									const details = document.querySelector('#product-quantity');
									const titleElement = document.querySelector('#product-title h1');
									const title = Array.from(titleElement.childNodes)
											.filter(node => node.nodeType === Node.TEXT_NODE)
											.map(node => node.textContent.trim())
											.join(' ')
											.replace(/\s+/g, ' ')
											.trim();
									const priceRaw = document.querySelector('#price')?.textContent.trim();
									const sold = parseInt(details?.querySelector('b')?.textContent.trim() || '0', 10);
									const available = details?.textContent.split(',')[0]?.trim();
									return { title, available, sold, price: priceRaw };
							})()
					`
							err := chromedp.Evaluate(js, &productDetailsResults).Do(ctx)
							if err != nil {
								return err
							}
							if productDetailsResults.Sold == 0 {
								return nil
							}
							mu.Lock()
							defer mu.Unlock()
							categoryUrl := productCategory.CategoryURL
							categoryID := extractCategoryID(categoryUrl)
							categoryName := extractCategoryName(categoryUrl)

							if _, exists := categoryNamesMap[categoryID]; !exists {
								categoryNamesMap[categoryID] = categoryName
							}

							if _, exists := categoryProductsMap[categoryID]; !exists {
								categoryProductsMap[categoryID] = make(map[string]entity.Product)
							}

							productID := extractProductID(productUrl)
							if _, exists := categoryProductsMap[categoryID][productID]; !exists {
								categoryProductsMap[categoryID][productID] = entity.Product{
									ProductID:    productID,
									ProductTitle: productDetailsResults.Title,
									ProductURL:   productUrl,
									ProductStock: productDetailsResults.Available,
									ProductPrice: productDetailsResults.Price,
									ProductSold:  productDetailsResults.Sold,
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

		// Arrange data, sort, and save it to a slice
		for categoryID, productMap := range categoryProductsMap {
			uniqueProducts := make(map[string]entity.Product)

			for _, product := range productMap {
				uniqueProducts[product.ProductID] = product
			}

			var products []entity.Product
			for _, product := range uniqueProducts {
				products = append(products, product)
			}

			if len(products) > 0 {
				sort.Slice(products, func(i, j int) bool {
					return products[i].ProductSold > products[j].ProductSold
				})
			}

			categoryName := categoryNamesMap[categoryID]
			allCategoryProducts = append(allCategoryProducts, entity.CategoryProducts{
				CategoryName: categoryName,
				CategoryID:   categoryID,
				Products:     products,
			})
		}

		message := "responseSuccess"
		scrapingcontroller.Producer.PublishScrapingData(message, scrapingcontroller.Rabbitmq.Channel, allCategoryProducts)
	}()

	<-done
}
