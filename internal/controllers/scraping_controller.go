package controllers

import (
	"context"
	"ecommerce-scraping-analytics/internal/config"
	"ecommerce-scraping-analytics/internal/entity"
	"ecommerce-scraping-analytics/internal/rabbitmq/producer"
	"fmt"
	"log"
	"math/rand"
	"regexp"
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

func (scrapingcontroller *ScrapingController) ScrapeSellerProduct(seller string) {
	// func extract id from url
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
	// get sellerCategory
	var allCategoryProducts []entity.CategoryProducts
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

	err := chromedp.Run(ctx,
		network.Enable(),
		network.SetExtraHTTPHeaders(network.Headers(headers)),
		chromedp.Navigate(sellerCategory),
		chromedp.Sleep(time.Duration(rand.Intn(1000)+1000)*time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var hrefs []string
			js := `
				(() => {
					const sections = document.querySelectorAll('section.clearfix');
					let allHrefs = [];
					sections.forEach(section => {
						const link = section.querySelector('a');
						if (link && link.href) {
							allHrefs.push(link.href);
						}
					});
					return allHrefs;
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
	// Scraping get list Product from each Categories
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
                        const elements = document.querySelectorAll('#product-list-grid > li > div.product-details > h2 > a');
                        return Array.from(elements).map(a => ({
                            href: a.href,
                            title: a.title
                        }));
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

			time.Sleep(2 * time.Second)
			cancel()
		}(url)
	}
	wg.Wait()

	// Scraping Each Product Details
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

		var productDetailsResults []struct {
			Available string `json:"available"`
			Sold      string `json:"sold"`
			Price     string `json:"price"`
		}

		err := chromedp.Run(tabCtx,
			network.Enable(),
			network.SetExtraHTTPHeaders(network.Headers(headers)),
			chromedp.Navigate(productUrl),
			chromedp.Sleep(3*time.Second),
			chromedp.ActionFunc(func(ctx context.Context) error {
				js := `
            (() => {
                const productPriceElements = document.querySelectorAll('#container > div > #content > #product-title > #product-title-actions > #product-title-actions > span > #price');
                const productDetailsElements = document.querySelectorAll('#container > div > #content > #product-image-container > #wBiggerImage > #product-details > #product-quantity');
                return Array.from(productDetailsElements).map(productDetailsElement => {
                    const available = productDetailsElement.querySelector('p')?.textContent.trim() || '';
                    const sold = productDetailsElement.querySelector('b')?.textContent.match(/\\d+/)?.[0] || '0';
                    const price = productPriceElements.length > 0 ? productPriceElements[0].textContent.trim() : '';
                    return { available, sold, price };
                });
            })()
            `
				return chromedp.Evaluate(js, &productDetailsResults).Do(ctx)
			}),
		)

		if err != nil {
			log.Printf("Error while scraping each product%s: %v\n", productUrl, err)
			cancel()
			continue
		}

		mu.Lock()
		for _, productResult := range productDetailsResults {
			Products = append(Products, entity.Product{
				ProductStock: productResult.Available,
				ProductPrice: productResult.Price,
				ProductSold:  productResult.Sold,
			})
		}
		mu.Unlock()
		cancel()
	}
	// // scrape review
	// // get reviewUrl
	// if len(Products) == 0 {
	// 	log.Println("No products available to extract review URL.")
	// 	return
	// }

	// var reviewURL string
	// selectedProduct := Products[0]

	// tabCtx, cancel := chromedp.NewContext(ctx)
	// defer cancel()

	// err = chromedp.Run(tabCtx,
	// 	network.Enable(),
	// 	network.SetExtraHTTPHeaders(network.Headers(headers)),
	// 	chromedp.Navigate(selectedProduct.ProductURL),
	// 	chromedp.Sleep(3*time.Second),
	// 	chromedp.ActionFunc(func(ctx context.Context) error {
	// 		js := `
	//       (() => {
	//           const reviewLinkElement = document.querySelector('a[href*="view-feedback"]');
	//           return reviewLinkElement ? reviewLinkElement.href : '';
	//       })()
	//       `
	// 		return chromedp.Evaluate(js, &reviewURL).Do(ctx)
	// 	}),
	// )

	// if err != nil {
	// 	log.Printf("Error while extracting review URL from product %s: %v\n", selectedProduct.ProductURL, err)
	// 	return
	// }

	// if reviewURL == "" {
	// 	log.Printf("No review URL found for product: %s\n", selectedProduct.ProductURL)
	// 	return
	// }
	// chromedp.Sleep(3 * time.Second)
	// // get review for each product
	// for _, product := range Products {
	// 	var reviewCounts struct {
	// 		Positive int `json:"positive"`
	// 		Neutral  int `json:"neutral"`
	// 		Negative int `json:"negative"`
	// 	}

	// 	err := chromedp.Run(tabCtx,
	// 		network.Enable(),
	// 		network.SetExtraHTTPHeaders(network.Headers(headers)),
	// 		chromedp.Navigate(reviewURL),
	// 		chromedp.Sleep(3*time.Second),
	// 		chromedp.ActionFunc(func(ctx context.Context) error {
	// 			productName := product.ProductTitle
	// 			js := `
	//           (() => {
	//               const table = document.querySelector('#comments > table');
	//               if (!table) return { positive: 0, neutral: 0, negative: 0 };

	//               const productNameElement = table.querySelector('tbody > tr:nth-child(2) > td:nth-child(3)');
	//               const reviews = Array.from(table.querySelectorAll('tbody > tr:nth-child(2) > td:nth-child(1) > i'));

	//               if (!productNameElement || productNameElement.textContent.trim() !== "` + productName + `") {
	//                   return { positive: 0, neutral: 0, negative: 0 };
	//               }

	//               let counts = { positive: 0, neutral: 0, negative: 0 };
	//               reviews.forEach(review => {
	//                   const reviewType = review.textContent.trim().toLowerCase();
	//                   if (reviewType.includes('positive')) counts.positive++;
	//                   else if (reviewType.includes('neutral')) counts.neutral++;
	//                   else if (reviewType.includes('negative')) counts.negative++;
	//               });

	//               return counts;
	//           })()
	//           `
	// 			return chromedp.Evaluate(js, &reviewCounts).Do(ctx)
	// 		}),
	// 	)

	// 	if err != nil {
	// 		log.Printf("Error while getting reviews for product %s: %v\n", product.ProductTitle, err)
	// 		continue
	// 	}
	// 	Products = append(Products, entity.Product{
	// 		PositiveRating: product.PositiveRating,
	// 		NegativeRating: product.NegativeRating,
	// 		NeutralRating:  product.NeutralRating,
	// 	})

	// }

	// Organizing Category Data
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
