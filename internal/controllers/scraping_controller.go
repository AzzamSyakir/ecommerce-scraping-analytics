package controllers

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"regexp"
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

type products struct {
	ProductID          string `json:"product_id"`
	ProductName        string `json:"product_name"`
	ProductURL         string `json:"product_url"`
	ProductPrice       string `json:"price"`
	ProductRating      string `json:"rating"`
	ProductReviewCount int    `json:"review_count"`
}
type CategoryProducts struct {
	CategoryID string     `json:"category_id"`
	Products   []products `json:"products"`
}

// func getProxies() []string {
// 	apiURL := "https://api.proxyscrape.com/v2/?request=displayproxies&protocol=http&timeout=10000&country=all&ssl=all&anonymity=all"
// 	resp, err := http.Get(apiURL)
// 	if err != nil {
// 		log.Fatalf("Failed to fetch proxies: %v", err)
// 	}
// 	defer resp.Body.Close()
// 	if resp.StatusCode != http.StatusOK {
// 		log.Fatalf("Failed to fetch proxies: status code %d", resp.StatusCode)
// 	}

// 	var proxies []string
// 	scanner := bufio.NewScanner(resp.Body)
// 	for scanner.Scan() {
// 		line := strings.TrimSpace(scanner.Text())
// 		if line != "" {
// 			proxies = append(proxies, "http://"+line)
// 		}
// 	}

// 	if err := scanner.Err(); err != nil {
// 		log.Fatalf("Error reading proxies: %v", err)
// 	}

// 	fmt.Printf("Fetched %d proxies from API\n", len(proxies))
// 	return proxies
// }

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
	var Product []products
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
	// // getProduct from sellerCategory

	var wg sync.WaitGroup
	var mu sync.Mutex
	var sellerProducts []string

	for _, url := range categoryURLs {
		wg.Add(1)

		go func(url string) {
			defer wg.Done()

			tabCtx, cancel := chromedp.NewContext(ctx)
			defer cancel()

			var productHrefs []string

			err := chromedp.Run(tabCtx,
				network.Enable(),
				network.SetExtraHTTPHeaders(network.Headers(headers)),
				chromedp.Navigate(url),
				chromedp.Sleep(3*time.Second),
				chromedp.ActionFunc(func(ctx context.Context) error {
					js := `
						(() => {
							const products = document.querySelectorAll('#product-list-grid > li > div.product-details > h2 > a');
							return Array.from(products).map(a => a.href);
						})()
					`
					return chromedp.Evaluate(js, &productHrefs).Do(ctx)
				}),
			)

			if err != nil {
				log.Printf("Error while scraping product from category %s: %v\n", url, err)
				return
			}

			mu.Lock()
			sellerProducts = append(sellerProducts, productHrefs...)
			mu.Unlock()

			log.Printf("Successfully scraped product from category: %s\n", url)
		}(url)

		time.Sleep(2 * time.Second)
	}
	wg.Wait()
	for _, productUrl := range sellerProducts {
		productId := extractProductID(productUrl)
		Product = append(Product, products{
			ProductID:  productId,
			ProductURL: productUrl,
		})
	}

	for i, url := range categoryURLs {
		fmt.Printf("Category %d: %s\n", i+1, url)
		categoryID := extractCategoryID(url)
		categoryProducts := append([]products{}, Product...)

		allCategoryProducts = append(allCategoryProducts, CategoryProducts{
			CategoryID: categoryID,
			Products:   categoryProducts,
		})
	}

	for _, category := range allCategoryProducts {
		fmt.Printf("Category ID: %s\n", category.CategoryID)
		if len(category.Products) == 0 {
			fmt.Println("\tNo products found for this category.")
		} else {
			for i, product := range category.Products {
				fmt.Printf("\tProduct %d URL: %s, Product ID: %s\n", i+1, product.ProductURL, product.ProductID)
			}
		}
		fmt.Println("Success Scraping Data")
	}
}
