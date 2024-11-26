package controllers

import (
	"context"
	"fmt"
	"log"
	"math/rand"
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
	ProductID          string `json:"product_id"`
	ProductName        string `json:"product_name"`
	ProductURL         string `json:"product_url"`
	ProductPrice       string `json:"price"`
	ProductRating      string `json:"rating"`
	ProductReviewCount int    `json:"review_count"`
}
type CategoryProducts struct {
	CategoryID string    `json:"category_id"`
	Products   []Product `json:"products"`
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
	// get sellerCategory
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
	sellerUrl := fmt.Sprintf(baseUrl, seller, "")
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

			for _, href := range hrefs {
				fullUrl := fmt.Sprintf("%s%s", sellerUrl, href)
				categoryURLs = append(categoryURLs, fullUrl)
			}
			return nil
		}),
	)

	if err != nil {
		log.Print("Error while performing getSellerCategory Details logic:", err)
	} else {
		for i, url := range categoryURLs {
			fmt.Printf("Category %d: %s\n", i+1, url)
		}
	}

	fmt.Println("Scraping sellerCategory URLs completed")
	// // getProduct from sellerCategory
	// var wg sync.WaitGroup

	// tabCtx, cancel := chromedp.NewContext(ctx)
	// defer cancel()

	// var sellerProducts []string

	// for _, url := range categoryURLs {
	// 	wg.Add(1)

	// 	go func(url string) {
	// 		defer wg.Done()

	// 		err := chromedp.Run(tabCtx,
	// 			network.Enable(),
	// 			network.SetExtraHTTPHeaders(network.Headers(headers)),
	// 			chromedp.Navigate(url),
	// 			chromedp.Sleep(5*time.Second),
	// 			chromedp.ActionFunc(func(ctx context.Context) error {
	// 				var nodes []*cdp.Node
	// 				err := chromedp.Nodes("#product-list-grid > li:nth-child(2) > a", &nodes, chromedp.ByQueryAll).Do(ctx)
	// 				if err != nil {
	// 					return fmt.Errorf("failed to extract nodes: %w", err)
	// 				}

	// 				for _, node := range nodes {
	// 					var href string
	// 					for i := 0; i < len(node.Attributes)-1; i += 2 {
	// 						if node.Attributes[i] == "href" {
	// 							href = node.Attributes[i+1]
	// 							break
	// 						}
	// 					}
	// 					if href != "" {
	// 						sellerProducts = append(sellerProducts, href)
	// 					}
	// 				}
	// 				return nil
	// 			}),
	// 		)

	// 		if err != nil {
	// 			log.Println("Error while scraping category:", url, err)
	// 		} else {
	// 			fmt.Println("Successfully scraped category:", url)
	// 		}
	// 	}(url)

	// 	time.Sleep(5 * time.Second)
	// }

	// wg.Wait()

	// fmt.Println("Seller Product URLs:")
	// for _, product := range sellerProducts {
	// 	fmt.Println(product)
	// }

	fmt.Println("Success Scraping Data")
}
