package controllers

import (
	"context"
	"ecommerce-scraping-analytics/internal/config"
	"ecommerce-scraping-analytics/internal/entity"
	"ecommerce-scraping-analytics/internal/model/response"
	"ecommerce-scraping-analytics/internal/rabbitmq/producer"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

type ScrapingController struct {
	Producer        *producer.ScrapingControllerProducer
	Rabbitmq        *config.RabbitMqConfig
	ScrapingProduct *ScrapingProduct
}
type ScrapingProduct struct {
	BrowserCtx  context.Context
	Seller      string
	Header      map[string]interface{}
	BlockedUrls []string
}

func NewScrapingController(rabbitMq *config.RabbitMqConfig, producer *producer.ScrapingControllerProducer) *ScrapingController {
	scrapingController := &ScrapingController{
		Producer: producer,
		Rabbitmq: rabbitMq,
	}
	return scrapingController
}
func NewScrapingProduct(browserCtx context.Context, seller string, headers map[string]interface{}) *ScrapingProduct {
	scrapingProduct := &ScrapingProduct{
		BrowserCtx:  browserCtx,
		Seller:      seller,
		Header:      headers,
		BlockedUrls: []string{"*.png", "*.jpg", "*.jpeg", "*.gif", "*.css", "*.js"},
	}
	return scrapingProduct
}

func (scrapingcontroller *ScrapingController) ScrapeAllSellerProducts(seller string) {
	fmt.Println("scrapeAllProduct start")
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
	// Channel and wg definitions
	categoryCh := make(chan string)
	productCh := make(chan entity.ProductWithCategory)
	retryProductDetailCh := make(chan entity.ProductWithCategory)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	// create and setup instance scrapingProduct
	newScrapingProduct := NewScrapingProduct(browserCtx, seller, headers)
	scrapingcontroller.ScrapingProduct = newScrapingProduct
	// goroutine handling error
	go func() {
		for err := range errCh {
			messageError := "responseError"
			messageCombine := messageError + ": " + err.Error()
			scrapingcontroller.Producer.PublishScrapingData(messageCombine, scrapingcontroller.Rabbitmq.Channel, nil)
		}
	}()
	//Scrape Categories
	wg.Add(1)
	go scrapingcontroller.ScrapeCategories(categoryCh, errCh, &wg)
	//Scrape ListProducts from Categories
	wg.Add(1)
	go scrapingcontroller.ScrapeListProducts(categoryCh, productCh, errCh, &wg)
	// Scrape Product Details
	var (
		mu                  sync.Mutex
		allCategoryProducts []entity.CategoryProducts
		categoryProductsMap = make(map[string]map[string]entity.Product)
		categoryNamesMap    = make(map[string]string)
	)
	wg.Add(1)
	go func() {
		defer func() {
			wg.Done()
		}()
		for prod := range productCh {
			wg.Add(1)
			go func(prod entity.ProductWithCategory) {
				productDetailCtx, cancel := chromedp.NewContext(browserCtx)
				defer func() {
					cancel()
					wg.Done()
				}()

				productUrl := prod.Product.ProductURL
				if strings.Contains(productUrl, ".ecrater.com") {
					if pos := strings.Index(productUrl, ".ecrater.com"); pos != -1 {
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
						checkService := `(() => document.querySelector('h1')?.textContent.trim())()`
						var pageTitle string
						if err := chromedp.Evaluate(checkService, &pageTitle).Do(ctx); err != nil {
							return err
						}
						if pageTitle == "Service Temporarily Unavailable" {
							retryProductDetailCh <- entity.ProductWithCategory{
								CategoryURL: prod.CategoryURL,
								Product: entity.Product{
									ProductID:  ExtractProductId(productUrl),
									ProductURL: productUrl,
								},
							}
							return nil
						}
						js := `
							(() => {
								const details = document.querySelector('#product-quantity');
								const titleElement = document.querySelector('#product-title h1');
								const title = titleElement ?
									Array.from(titleElement.childNodes)
										.filter(node => node.nodeType === Node.TEXT_NODE)
										.map(node => node.textContent.trim())
										.join(' ')
										.replace(/\s+/g, ' ')
										.trim() : '';
								const priceRaw = document.querySelector('#price')?.textContent.trim();
								const sold = details ? 
									parseInt(details.querySelector('b')?.textContent.trim() || '0', 10) : 0;
								const available = details ? details.textContent.split(',')[0]?.trim() : '';
								return { title, available, sold, price: priceRaw };
							})()
						`
						if err := chromedp.Evaluate(js, &productDetailsResults).Do(ctx); err != nil {
							return err
						}
						return nil
					}),
				)
				if err != nil {
					errCh <- err
					return
				}

				mu.Lock()
				categoryUrl := prod.CategoryURL
				categoryID := ExtractCategoryId(categoryUrl)
				categoryName := ExtractCategoryName(categoryUrl)
				if _, exists := categoryNamesMap[categoryID]; !exists {
					categoryNamesMap[categoryID] = categoryName
				}
				if _, exists := categoryProductsMap[categoryID]; !exists {
					categoryProductsMap[categoryID] = make(map[string]entity.Product)
				}
				productID := ExtractProductId(productUrl)
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
				mu.Unlock()
			}(prod)
		}
		go func() {
			for retryProd := range retryProductDetailCh {
				wg.Add(1)
				go func(prod entity.ProductWithCategory) {
					retryCtx, cancel := chromedp.NewContext(browserCtx)
					productUrl := prod.Product.ProductURL
					defer func() {
						cancel()
						wg.Done()
					}()
					var productDetailsResults struct {
						Title     string `json:"title"`
						Available string `json:"available"`
						Sold      int    `json:"sold"`
						Price     string `json:"price"`
					}
					err := chromedp.Run(retryCtx,
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
										const title = titleElement ?
											Array.from(titleElement.childNodes)
												.filter(node => node.nodeType === Node.TEXT_NODE)
												.map(node => node.textContent.trim())
												.join(' ')
												.replace(/\s+/g, ' ')
												.trim() : '';
										const priceRaw = document.querySelector('#price')?.textContent.trim();
										const sold = details ? 
											parseInt(details.querySelector('b')?.textContent.trim() || '0', 10) : 0;
										const available = details ? details.textContent.split(',')[0]?.trim() : '';
										return { title, available, sold, price: priceRaw };
									})()
								`
							return chromedp.Evaluate(js, &productDetailsResults).Do(ctx)
						}),
					)
					if err != nil {
						errCh <- err
						return
					}

					if productDetailsResults.Sold == 0 {
						return
					}

					mu.Lock()
					categoryUrl := prod.CategoryURL
					categoryID := ExtractCategoryId(categoryUrl)
					categoryName := ExtractCategoryName(categoryUrl)
					if _, exists := categoryNamesMap[categoryID]; !exists {
						categoryNamesMap[categoryID] = categoryName
					}
					if _, exists := categoryProductsMap[categoryID]; !exists {
						categoryProductsMap[categoryID] = make(map[string]entity.Product)
					}
					productID := ExtractProductId(productUrl)
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
					mu.Unlock()
				}(retryProd)
			}
		}()
	}()

	// wait for all goroutine to finish
	wg.Wait()
	close(errCh)
	close(retryProductDetailCh)
	cancelBrowser()
	// Arrange data, sort, and save it to a slice
	var totalItemsSold, totalProductsSold int
	for categoryID, productMap := range categoryProductsMap {
		uniqueProducts := make(map[string]entity.Product)

		for _, product := range productMap {
			uniqueProducts[product.ProductID] = product
		}

		var products []entity.Product
		var itemsSold, productsSoldCount int

		for _, product := range uniqueProducts {
			products = append(products, product)
			itemsSold += product.ProductSold
			if product.ProductSold > 0 {
				productsSoldCount++
			}
		}

		if len(products) > 0 {
			sort.Slice(products, func(i, j int) bool {
				return products[i].ProductSold > products[j].ProductSold
			})
		}

		categoryName := categoryNamesMap[categoryID]
		allCategoryProducts = append(allCategoryProducts, entity.CategoryProducts{
			CategoryName:      categoryName,
			CategoryID:        categoryID,
			Products:          products,
			ItemsSold:         itemsSold,
			ProductsSoldCount: productsSoldCount,
		})

		totalItemsSold += itemsSold
		totalProductsSold += productsSoldCount
	}

	responseData := &response.SellerProductResponse{
		Categories:        allCategoryProducts,
		ItemsSold:         totalItemsSold,
		ProductsSoldCount: totalProductsSold,
	}
	message := "responseSuccess"
	scrapingcontroller.Producer.PublishScrapingData(message, scrapingcontroller.Rabbitmq.Channel, responseData)
}
func (scrapingcontroller *ScrapingController) ScrapeSoldSellerProducts(seller string) {
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
	// Channel and wg definitions
	categoryCh := make(chan string)
	productCh := make(chan entity.ProductWithCategory)
	retryProductDetailCh := make(chan entity.ProductWithCategory)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	// create and setup instance scrapingProduct
	newScrapingProduct := NewScrapingProduct(browserCtx, seller, headers)
	scrapingcontroller.ScrapingProduct = newScrapingProduct
	// goroutine handling error
	go func() {
		for err := range errCh {
			messageError := "responseError"
			messageCombine := messageError + ": " + err.Error()
			scrapingcontroller.Producer.PublishScrapingData(messageCombine, scrapingcontroller.Rabbitmq.Channel, nil)
		}
	}()
	//Scrape Categories
	wg.Add(1)
	go scrapingcontroller.ScrapeCategories(categoryCh, errCh, &wg)
	//Scrape ListProducts from Categories
	wg.Add(1)
	go scrapingcontroller.ScrapeListProducts(categoryCh, productCh, errCh, &wg)
	// Scrape Product Details
	var (
		mu                  sync.Mutex
		allCategoryProducts []entity.CategoryProducts
		categoryProductsMap = make(map[string]map[string]entity.Product)
		categoryNamesMap    = make(map[string]string)
	)
	wg.Add(1)
	go func() {
		defer func() {
			wg.Done()
		}()

		for prod := range productCh {
			wg.Add(1)
			go func(prod entity.ProductWithCategory) {
				productDetailCtx, cancel := chromedp.NewContext(browserCtx)
				defer func() {
					cancel()
					wg.Done()
				}()

				productUrl := prod.Product.ProductURL
				if strings.Contains(productUrl, ".ecrater.com") {
					if pos := strings.Index(productUrl, ".ecrater.com"); pos != -1 {
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
						checkService := `(() => document.querySelector('h1')?.textContent.trim())()`
						var pageTitle string
						if err := chromedp.Evaluate(checkService, &pageTitle).Do(ctx); err != nil {
							return err
						}
						if pageTitle == "Service Temporarily Unavailable" {
							retryProductDetailCh <- entity.ProductWithCategory{
								CategoryURL: prod.CategoryURL,
								Product: entity.Product{
									ProductID:  ExtractProductId(productUrl),
									ProductURL: productUrl,
								},
							}
							return nil
						}
						js := `
							(() => {
								const details = document.querySelector('#product-quantity');
								const titleElement = document.querySelector('#product-title h1');
								const title = titleElement ?
									Array.from(titleElement.childNodes)
										.filter(node => node.nodeType === Node.TEXT_NODE)
										.map(node => node.textContent.trim())
										.join(' ')
										.replace(/\s+/g, ' ')
										.trim() : '';
								const priceRaw = document.querySelector('#price')?.textContent.trim();
								const sold = details ? 
									parseInt(details.querySelector('b')?.textContent.trim() || '0', 10) : 0;
								const available = details ? details.textContent.split(',')[0]?.trim() : '';
								return { title, available, sold, price: priceRaw };
							})()
						`
						if err := chromedp.Evaluate(js, &productDetailsResults).Do(ctx); err != nil {
							return err
						}
						return nil
					}),
				)
				if err != nil {
					errCh <- err
					return
				}

				if productDetailsResults.Sold == 0 {
					return
				}

				mu.Lock()
				categoryUrl := prod.CategoryURL
				categoryID := ExtractCategoryId(categoryUrl)
				categoryName := ExtractCategoryName(categoryUrl)
				if _, exists := categoryNamesMap[categoryID]; !exists {
					categoryNamesMap[categoryID] = categoryName
				}
				if _, exists := categoryProductsMap[categoryID]; !exists {
					categoryProductsMap[categoryID] = make(map[string]entity.Product)
				}
				productID := ExtractProductId(productUrl)
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
				mu.Unlock()
			}(prod)
		}
		go func() {
			for retryProd := range retryProductDetailCh {
				wg.Add(1)
				go func(prod entity.ProductWithCategory) {
					retryCtx, cancel := chromedp.NewContext(browserCtx)
					productUrl := prod.Product.ProductURL
					defer func() {
						cancel()
						wg.Done()
					}()
					var productDetailsResults struct {
						Title     string `json:"title"`
						Available string `json:"available"`
						Sold      int    `json:"sold"`
						Price     string `json:"price"`
					}
					err := chromedp.Run(retryCtx,
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
										const title = titleElement ?
											Array.from(titleElement.childNodes)
												.filter(node => node.nodeType === Node.TEXT_NODE)
												.map(node => node.textContent.trim())
												.join(' ')
												.replace(/\s+/g, ' ')
												.trim() : '';
										const priceRaw = document.querySelector('#price')?.textContent.trim();
										const sold = details ? 
											parseInt(details.querySelector('b')?.textContent.trim() || '0', 10) : 0;
										const available = details ? details.textContent.split(',')[0]?.trim() : '';
										return { title, available, sold, price: priceRaw };
									})()
								`
							return chromedp.Evaluate(js, &productDetailsResults).Do(ctx)
						}),
					)
					if err != nil {
						errCh <- err
						return
					}

					if productDetailsResults.Sold == 0 {
						return
					}

					mu.Lock()
					categoryUrl := prod.CategoryURL
					categoryID := ExtractCategoryId(categoryUrl)
					categoryName := ExtractCategoryName(categoryUrl)
					if _, exists := categoryNamesMap[categoryID]; !exists {
						categoryNamesMap[categoryID] = categoryName
					}
					if _, exists := categoryProductsMap[categoryID]; !exists {
						categoryProductsMap[categoryID] = make(map[string]entity.Product)
					}
					productID := ExtractProductId(productUrl)
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
					mu.Unlock()
				}(retryProd)
			}
		}()
	}()

	// wait for all goroutine to finish
	wg.Wait()
	close(errCh)
	close(retryProductDetailCh)
	cancelBrowser()
	// Arrange data, sort, and save it to a slice
	var totalItemsSold, totalProductsSold int
	for categoryID, productMap := range categoryProductsMap {
		uniqueProducts := make(map[string]entity.Product)

		for _, product := range productMap {
			uniqueProducts[product.ProductID] = product
		}

		var products []entity.Product
		var itemsSold, productsSoldCount int

		for _, product := range uniqueProducts {
			products = append(products, product)
			itemsSold += product.ProductSold
			if product.ProductSold > 0 {
				productsSoldCount++
			}
		}

		if len(products) > 0 {
			sort.Slice(products, func(i, j int) bool {
				return products[i].ProductSold > products[j].ProductSold
			})
		}

		categoryName := categoryNamesMap[categoryID]
		allCategoryProducts = append(allCategoryProducts, entity.CategoryProducts{
			CategoryName:      categoryName,
			CategoryID:        categoryID,
			Products:          products,
			ItemsSold:         itemsSold,
			ProductsSoldCount: productsSoldCount,
		})

		totalItemsSold += itemsSold
		totalProductsSold += productsSoldCount
	}

	responseData := &response.SellerProductResponse{
		Categories:        allCategoryProducts,
		ItemsSold:         totalItemsSold,
		ProductsSoldCount: totalProductsSold,
	}
	message := "responseSuccess"
	scrapingcontroller.Producer.PublishScrapingData(message, scrapingcontroller.Rabbitmq.Channel, responseData)
}
func (scrapingController *ScrapingController) ScrapeCategories(categoryCh chan string, errCh chan error, wg *sync.WaitGroup) {
	var categoryURLs []string
	defer func() {
		close(categoryCh)
		wg.Done()
	}()
	var wgCategory sync.WaitGroup
	baseUrl := "http://%s.ecrater.com%s"
	sellerCategory := fmt.Sprintf(baseUrl, scrapingController.ScrapingProduct.Seller, "/category.php")
	wgCategory.Add(1)
	go func() {
		defer wgCategory.Done()
		err := chromedp.Run(scrapingController.ScrapingProduct.BrowserCtx,
			network.SetCacheDisabled(false),
			network.SetBlockedURLS(scrapingController.ScrapingProduct.BlockedUrls),
			network.Enable(),
			network.SetExtraHTTPHeaders(network.Headers(scrapingController.ScrapingProduct.Header)),
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
	}()
	wgCategory.Wait()
}
func (scrapingController *ScrapingController) ScrapeListProducts(categoryCh chan string, productCh chan entity.ProductWithCategory, errCh chan error, wg *sync.WaitGroup) {
	var productListWg sync.WaitGroup
	defer func() {
		wg.Done()
		productListWg.Wait()
		close(productCh)
	}()
	// scrape list product from categories
	for url := range categoryCh {
		categoryUrl := fmt.Sprintf("%s?&perpage=80", url)
		productListWg.Add(1)
		go func(url string) {
			var categoryProductResults []struct {
				Href string `json:"href"`
			}
			productCategoriesCtx, cancel := chromedp.NewContext(scrapingController.ScrapingProduct.BrowserCtx)
			defer func() {
				productListWg.Done()
				cancel()
			}()
			url = fmt.Sprintf("%s?&perpage=80", url)
			err := chromedp.Run(productCategoriesCtx,
				network.SetCacheDisabled(false),
				network.SetBlockedURLS([]string{"*.png", "*.jpg", "*.jpeg", "*.gif", "*.css", "*.js"}),
				network.Enable(),
				network.SetExtraHTTPHeaders(network.Headers(scrapingController.ScrapingProduct.Header)),
				chromedp.Navigate(url),
				chromedp.WaitReady("body"),
				chromedp.ActionFunc(func(ctx context.Context) error {

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
					if err := chromedp.Evaluate(jsScrapeProduct, &categoryProductResults).Do(ctx); err != nil {
						return err
					}
					return nil
				}))
			if err != nil {
				errCh <- err
				return
			}
			duplicate := make(map[string]bool)
			for _, productResult := range categoryProductResults {
				if duplicate[productResult.Href] {
					continue
				}
				duplicate[productResult.Href] = true

				productCh <- entity.ProductWithCategory{
					CategoryURL: url,
					Product: entity.Product{
						ProductID:  ExtractProductId(productResult.Href),
						ProductURL: productResult.Href,
					},
				}
			}
		}(categoryUrl)
	}
}

// extract function
func ExtractProductId(url string) string {
	re := regexp.MustCompile(`/p/([0-9]+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}
func ExtractCategoryName(url string) string {
	cleanURL := strings.Split(url, "?")[0]

	re := regexp.MustCompile(`/c/[^/]+/([^/]+)`)
	matches := re.FindStringSubmatch(cleanURL)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}
func ExtractCategoryId(url string) string {
	cleanURL := strings.Split(url, "?")[0]

	re := regexp.MustCompile(`/c/([0-9]+)`)
	matches := re.FindStringSubmatch(cleanURL)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}
