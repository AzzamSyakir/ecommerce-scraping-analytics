package controllers

import (
	"context"
	"ecommerce-scraping-analytics/internal/config"
	"ecommerce-scraping-analytics/internal/entity"
	"ecommerce-scraping-analytics/internal/rabbitmq/producer"
	"fmt"
	"log"
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

	headers := map[string]interface{}{
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
		"Accept-Encoding":           "gzip, deflate, br",
		"Accept-Language":           "en-US,en;q=0.5",
		"Cache-Control":             "max-age=0",
		"Connection":                "keep-alive",
		"Upgrade-Insecure-Requests": "1",
		"User-Agent":                "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.6778.69 Safari/537.36",
		"Referer":                   "https://www.etsy.com",
		"Sec-CH-UA":                 "\"Google Chrome\";v=\"131\", \"Chromium\";v=\"131\", \"Not_A Brand\";v=\"99\"",
		"Sec-CH-UA-Mobile":          "?0",
		"Sec-CH-UA-Platform":        "\"Linux\"",
	}

	ctx, cancel := chromedp.NewExecAllocator(context.Background(), append(chromedp.DefaultExecAllocatorOptions[:], chromedp.Flag("headless", false))...)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()

	var categoryURLs []string
	baseUrl := "http://www.%s.ecrater.com%s"
	sellerCategory := fmt.Sprintf(baseUrl, seller, "/category.php")

	// SECTION: Scraping Seller Categories
	err := chromedp.Run(ctx,
		network.Enable(),
		network.SetExtraHTTPHeaders(network.Headers(headers)),
		chromedp.Navigate(sellerCategory),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var hrefs []string
			js := `
			(() => {
				return Array.from(document.querySelectorAll('section.clearfix a[href]'))
								.map(link => link.href);
			})()
			`
			err := chromedp.Evaluate(js, &hrefs).Do(ctx)
			if err != nil {
				return err
			}
			categoryURLs = append(categoryURLs, hrefs...)
			return nil
		}),
	)

	if err != nil {
		log.Print("Error while performing getSellerCategory Details logic:", err)
	}

	fmt.Println("Scraping sellerCategory URLs completed")

	// SECTION: Scraping Products from Each Category
	var wg sync.WaitGroup
	var mu sync.Mutex
	var Products []entity.Product
	for _, url := range categoryURLs {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			tabCtx, cancel := chromedp.NewContext(ctx)
			defer cancel()

			var categoryProductResults []struct {
				Href  string `json:"href"`
				Title string `json:"title"`
			}

			err := chromedp.Run(tabCtx,
				network.Enable(),
				network.SetExtraHTTPHeaders(network.Headers(headers)),
				chromedp.Navigate(url),
				chromedp.WaitReady("#product-list-grid"),
				chromedp.ActionFunc(func(ctx context.Context) error {
					js := `
                   (() => {
						return [...document.querySelectorAll('#product-list-grid > li > div.product-details > h2 > a')]
							.map(a => ({ href: a.href, title: a.title }));
					})()
					`
					return chromedp.Evaluate(js, &categoryProductResults).Do(ctx)
				}),
			)

			if err != nil {
				log.Printf("Error while scraping product from category %s: %v\n", url, err)
				return
			}

			mu.Lock()
			for _, productResult := range categoryProductResults {
				Products = append(Products, entity.Product{
					ProductID:    extractProductID(productResult.Href),
					ProductTitle: productResult.Title,
					ProductURL:   productResult.Href,
				})
			}
			mu.Unlock()
		}(url)
	}
	wg.Wait()

	// SECTION: Scraping Details of Each Product
	for _, product := range Products {
		var productUrl string
		if strings.Contains(product.ProductURL, ".ecrater.com") {
			pos := strings.Index(product.ProductURL, ".ecrater.com")
			if pos != -1 {
				productUrl = "https://www.ecrater.com" + product.ProductURL[pos+len(".ecrater.com"):]
			}
		}

		tabCtx, cancel := chromedp.NewContext(ctx)
		defer cancel()

		var productDetailsResults struct {
			Available string `json:"available"`
			Sold      string `json:"sold"`
			Price     string `json:"price"`
		}

		err := chromedp.Run(tabCtx,
			network.Enable(),
			network.SetExtraHTTPHeaders(network.Headers(headers)),
			chromedp.Navigate(productUrl),
			chromedp.ActionFunc(func(ctx context.Context) error {
				js := `
				(() => {
					const priceElement = document.querySelector('#container #content #product-title-actions #price');
					const detailsElement = document.querySelector('#container #content #product-details');

					const price = priceElement?.textContent.trim() || '';
					const available = detailsElement?.querySelector('p')?.textContent.trim() || '';
					const soldMatch = detailsElement?.querySelector('b')?.textContent.match(/\d+/);
					const sold = soldMatch ? soldMatch[0] : '0';

					return { available, sold, price };
				})()
				`
				return chromedp.Evaluate(js, &productDetailsResults).Do(ctx)
			}),
		)

		if err != nil {
			log.Printf("Error while scraping each product%s: %v\n", productUrl, err)
			continue
		}

		mu.Lock()
		Products = append(Products, entity.Product{
			ProductStock: productDetailsResults.Available,
			ProductPrice: productDetailsResults.Price,
			ProductSold:  productDetailsResults.Sold,
		})
		mu.Unlock()
	}

	// SECTION: Organizing Data
	var allCategoryProducts []entity.CategoryProducts
	for _, url := range categoryURLs {
		allCategoryProducts = append(allCategoryProducts, entity.CategoryProducts{
			CategoryName: extractCategoryName(url),
			CategoryID:   extractCategoryID(url),
			Products:     Products,
		})
	}

	message := "responseSuccess"
	scrapingcontroller.Producer.PublishScrapingData(message, scrapingcontroller.Rabbitmq.Channel, allCategoryProducts)

	fmt.Println("Success Scraping Data")
}
