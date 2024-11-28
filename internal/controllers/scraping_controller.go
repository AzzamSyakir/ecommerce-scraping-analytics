package controllers

import (
	"context"
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

type ScrapingController struct{}

func NewScrapingController() *ScrapingController {
	scrapingController := &ScrapingController{}
	return scrapingController
}

type Product struct {
	ProductID      string `json:"product_id"`
	ProductTitle   string `json:"product_name"`
	ProductURL     string `json:"product_url"`
	ProductPrice   string `json:"price"`
	ProductStock   string `json:"product_stock"`
	ProductSold    string `json:"product_sold"`
	PositiveRating int    `json:"positive_rating"`
	NeutralRating  int    `json:"neutral_rating"`
	NegativeRating int    `json:"negative_rating"`
}

type CategoryProducts struct {
	CategoryID   string    `json:"category_id"`
	CategoryName string    `json:"category_name"`
	Products     []Product `json:"products"`
}

func (scrapingcontroller *ScrapingController) ScrapePopularProductsBySeller(seller string) {
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
	// get sellerCategory
	var allCategoryProducts []CategoryProducts
	var Products []Product
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
	// Scraping Product from Categories
	var wg sync.WaitGroup
	var mu sync.Mutex
	var productResults []struct {
		Href      string `json:"href"`
		Title     string `json:"title"`
		Price     string `json:"price"`
		Sold      string `json:"sold"`
		Available string `json:"available"`
	}

	for _, url := range categoryURLs {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()

			tabCtx, cancel := chromedp.NewContext(ctx)
			defer cancel()

			err := chromedp.Run(tabCtx,
				network.Enable(),
				network.SetExtraHTTPHeaders(network.Headers(headers)),
				chromedp.Navigate(url),
				chromedp.Sleep(3*time.Second),
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
					return chromedp.Evaluate(js, &productResults).Do(ctx)
				}),
			)

			if err != nil {
				log.Printf("Error while scraping product from category %s: %v\n", url, err)
				return
			}

			mu.Lock()
			for _, productResult := range productResults {
				Products = append(Products, Product{
					ProductID:    extractProductID(productResult.Href),
					ProductStock: productResult.Available,
					ProductTitle: productResult.Title,
					ProductPrice: productResult.Price,
					ProductSold:  productResult.Sold,
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
				return chromedp.Evaluate(js, &productResults).Do(ctx)
			}),
		)

		if err != nil {
			log.Printf("Error while scraping each product%s: %v\n", productUrl, err)
			cancel()
			continue
		}

		mu.Lock()
		for _, productResult := range productResults {
			Products = append(Products, Product{
				ProductID:    extractProductID(productResult.Href),
				ProductStock: productResult.Available,
				ProductTitle: productResult.Title,
				ProductPrice: productResult.Price,
				ProductSold:  productResult.Sold,
				ProductURL:   productResult.Href,
			})
		}
		mu.Unlock()
		cancel()
	}
	// scrape review
	if len(Products) > 0 {
		var reviewURL string
		selectedProduct := Products[0]

		tabCtx, cancel := chromedp.NewContext(ctx)
		defer cancel()

		err := chromedp.Run(tabCtx,
			network.Enable(),
			network.SetExtraHTTPHeaders(network.Headers(headers)),
			chromedp.Navigate(selectedProduct.ProductURL),
			chromedp.Sleep(3*time.Second),
			chromedp.ActionFunc(func(ctx context.Context) error {
				js := `
            (() => {
                const reviewLinkElement = document.querySelector('a[href*="view-feedback"]');
                return reviewLinkElement ? reviewLinkElement.href : '';
            })()
            `
				return chromedp.Evaluate(js, &reviewURL).Do(ctx)
			}),
		)

		if err != nil {
			log.Printf("Error while extracting review URL from product %s: %v\n", selectedProduct.ProductURL, err)
		} else if reviewURL != "" {
			log.Printf("Successfully extracted review URL for store: %s\n", reviewURL)
		} else {
			log.Printf("No review URL found for product: %s\n", selectedProduct.ProductURL)
		}
	} else {
		log.Println("No products available to extract review URL.")
	}

	// Organizing Category Data
	for _, url := range categoryURLs {
		categoryID := extractCategoryID(url)
		categoryProducts := append([]Product{}, Products...)

		_ = append(allCategoryProducts, CategoryProducts{
			CategoryID: categoryID,
			Products:   categoryProducts,
		})
	}

	// Displaying Scraped Data
	// for i, category := range allCategoryProducts {
	// 	fmt.Printf("Category %d:\n", i+1)
	// 	fmt.Printf("\tCategory ID: %s\n", category.CategoryID)
	// 	fmt.Printf("\tCategory Name: %s\n", category.CategoryName)

	// 	if len(category.Products) == 0 {
	// 		fmt.Println("\tNo products found for this category.")
	// 	} else {
	// 		fmt.Println("\tProducts:")
	// 		for j, product := range category.Products {
	// 			fmt.Printf("\t\tProduct %d:\n", j+1)
	// 			fmt.Printf("\t\t\tProduct ID: %s\n", product.ProductID)
	// 			fmt.Printf("\t\t\tProduct Title: %s\n", product.ProductTitle)
	// 			fmt.Printf("\t\t\tProduct URL: %s\n", product.ProductURL)
	// 			fmt.Printf("\t\t\tProduct Price: %s\n", product.ProductPrice)
	// 			fmt.Printf("\t\t\tProduct Stock: %s\n", product.ProductStock)
	// 			fmt.Printf("\t\t\tProduct Sold: %s\n", product.ProductSold)
	// 			fmt.Printf("\t\t\tPositive Rating: %d\n", product.PositiveRating)
	// 			fmt.Printf("\t\t\tNeutral Rating: %d\n", product.NeutralRating)
	// 			fmt.Printf("\t\t\tNegative Rating: %d\n", product.NegativeRating)
	// 		}
	// 	}
	// }
	fmt.Println("Success Scraping Data")
}
