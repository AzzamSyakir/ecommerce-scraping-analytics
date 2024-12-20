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
		"Accept-Language":           "en-US,en;q=0.5",
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
			chromedp.Flag("headless", false),
		)...,
	)
	defer cancel()

	// Create a browser context from the allocator
	browserCtx, cancelBrowser := chromedp.NewContext(ctx)
	defer cancelBrowser()
	// Channel and pool definitions
	maxWorker := 100
	categoryCh := make(chan string)
	productCh := make(chan entity.Product)
	done := make(chan bool)
	categoryPool := make(chan struct{}, maxWorker)
	productListpool := make(chan struct{}, maxWorker)
	productDetailPool := make(chan struct{}, maxWorker)
	retryCh := make(chan string, maxWorker)
	retryPool := make(chan struct{}, maxWorker)
	errCh := make(chan error, 1)

	// handling error
	var once sync.Once
	go func() {
		for err := range errCh {
			messageError := "responseError"
			messageCombine := messageError + ": " + err.Error()
			scrapingcontroller.Producer.PublishScrapingData(messageCombine, scrapingcontroller.Rabbitmq.Channel, nil)
			once.Do(func() {
				cancelBrowser()
				close(retryCh)
				close(done)
				close(productDetailPool)
				close(errCh)
			})
		}
	}()

	//Scrape Categories
	var categoryURLs []string
	go func() {
		defer close(categoryPool)
		defer close(categoryCh)
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
				js := `
			(() => {
				return Array.from(document.querySelectorAll('section.clearfix a[href]'))
								.map(link => link.href);
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
	}()
	//Scrape Products from Categories
	go func() {
		var wg sync.WaitGroup
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

				var categoryProductResults []struct {
					Href string `json:"href"`
				}

				err := chromedp.Run(productCategoriesCtx,
					network.SetCacheDisabled(false),
					network.SetBlockedURLS([]string{"*.png", "*.jpg", "*.jpeg", "*.gif", "*.css", "*.js"}),
					network.Enable(),
					network.SetExtraHTTPHeaders(network.Headers(headers)),
					chromedp.Navigate(url),
					chromedp.WaitReady("#product-list-grid"),
					chromedp.ActionFunc(func(ctx context.Context) error {
						js := `
					(() => {
						return [...document.querySelectorAll('#product-list-grid > li > div.product-details > h2 > a')]
							.map(a => ({ href: a.href}));
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
							const h1Element = document.querySelector('body > h1');
							return h1Element ? h1Element.textContent.trim() : '';
						})();
					`

						var pageTitle string
						if err := chromedp.Evaluate(checkService, &pageTitle).Do(ctx); err != nil {
							return err
						}

						if pageTitle == "Service Temporarily Unavailable" {
							retryCh <- productUrl
							return nil
						}

						js := `
						(() => {
							const detailsElement = document.querySelector('#product-quantity');
							const priceRaw = document.querySelector('#price')?.textContent.trim() || '';
							const title = document.querySelector('#product-title > h1')?.textContent.trim() || '';
							const price = priceRaw ? '$' + priceRaw : '';
							const available = detailsElement?.textContent.split(',')[0]?.trim() || '';
							const sold = detailsElement?.querySelector('b')?.textContent.trim() || '0';
							return { title, available, sold, price };
						})();
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

		// Goroutine untuk memproses retry
		go func() {
			for productUrl := range retryCh {
				fmt.Println("retry error cihuuyy")
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

					// Proses ulang scraping produk
					err := chromedp.Run(browserCtx,
						chromedp.Navigate(url),
						chromedp.ActionFunc(func(ctx context.Context) error {
							js := `
						(() => {
							const detailsElement = document.querySelector('#product-quantity');
							const priceRaw = document.querySelector('#price')?.textContent.trim() || '';
							const title = document.querySelector('#product-title > h1')?.textContent.trim() || '';
							const price = priceRaw ? '$' + priceRaw : '';
							const available = detailsElement?.textContent.split(',')[0]?.trim() || '';
							const sold = detailsElement?.querySelector('b')?.textContent.trim() || '0';
							return { title, available, sold, price };
						})();
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
						fmt.Println("eror di err retry cihuyy ", err)
						errCh <- err
						return
					}
				}(productUrl)
			}
		}()

		// Tunggu semua goroutine selesai
		retryWg.Wait()
		wg.Wait()

		// Tutup semua channel
		close(retryCh)
		close(done)
		close(productDetailPool)
		close(errCh)

		// Simpan hasil scraping
		for _, url := range categoryURLs {
			allCategoryProducts = append(allCategoryProducts, entity.CategoryProducts{
				CategoryName: extractCategoryName(url),
				CategoryID:   extractCategoryID(url),
				Products:     products,
			})
		}

		// Kirim hasil ke RabbitMQ
		message := "responseSuccess"
		scrapingcontroller.Producer.PublishScrapingData(message, scrapingcontroller.Rabbitmq.Channel, allCategoryProducts)
	}()

	<-done
}
