package controllers

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
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

func getProxies() []string {
	apiURL := "https://api.proxyscrape.com/v2/?request=displayproxies&protocol=http&timeout=10000&country=all&ssl=all&anonymity=all"
	resp, err := http.Get(apiURL)
	if err != nil {
		log.Fatalf("Failed to fetch proxies: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Failed to fetch proxies: status code %d", resp.StatusCode)
	}

	var proxies []string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			proxies = append(proxies, "http://"+line)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading proxies: %v", err)
	}

	fmt.Printf("Fetched %d proxies from API\n", len(proxies))
	return proxies
}

func (scrapingcontroller *ScrapingController) ProductCategoryTrendsScrapingController() {
	proxies := getProxies()
	if len(proxies) == 0 {
		log.Fatal("No proxies available")
	}
	homepageProxy := proxies[rand.Intn(len(proxies))]
	fmt.Println("homeProxy", homepageProxy)
	// get categories detail
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

	ctx, cancel := chromedp.NewExecAllocator(context.Background(), append(chromedp.DefaultExecAllocatorOptions[:], chromedp.Flag("headless", false), chromedp.ProxyServer(homepageProxy))...)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()

	var categoryURLs []string
	baseUrl := "https://www.etsy.com/"

	err := chromedp.Run(ctx,
		network.Enable(),
		network.SetExtraHTTPHeaders(network.Headers(headers)),
		chromedp.Navigate(baseUrl),
		chromedp.Sleep(time.Duration(rand.Intn(1000)+1000)*time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var iframeExists bool
			chromedp.Evaluate(`document.querySelector("iframe[src*='geo.captcha-delivery.com']") !== null`, &iframeExists).Do(ctx)

			if iframeExists {
				fmt.Println("Captcha detected, reloading page...")
				err := chromedp.Reload().Do(ctx)
				if err != nil {
					log.Println("Error reloading page:", err)
					return nil
				}
				log.Println("Page reloaded due to captcha, continuing scraping...")
			} else {
				fmt.Println("No captcha detected, proceeding with scraping...")
			}
			var nodes []*cdp.Node
			err := chromedp.Nodes("div[role='menu'] a", &nodes, chromedp.ByQueryAll).Do(ctx)
			if err != nil {
				return fmt.Errorf("failed to query nodes: %w", err)
			}

			ch := make(chan string, len(nodes))

			for _, node := range nodes {
				go func(node *cdp.Node) {
					var href string
					for i := 0; i < len(node.Attributes)-1; i += 2 {
						if node.Attributes[i] == "href" {
							href = node.Attributes[i+1]
							break
						}
					}
					if href == "" {
						log.Println("Failed to extract href attribute")
						ch <- ""
					} else {
						ch <- href
					}
				}(node)
			}

			for i := 0; i < len(nodes); i++ {
				href := <-ch
				if href != "" {
					fullURL := fmt.Sprintf("%s%s", baseUrl, href)
					categoryURLs = append(categoryURLs, fullURL)
				}
			}

			return nil
		}),
	)
	fmt.Println("Scraping Category URLs completed")
	for i, url := range categoryURLs {
		fmt.Printf("Category %d: %s\n", i+1, url)
	}
	if err != nil {
		log.Print("Error while performing getCategory Details logic:", err)
	}
	// get product category
	// var wg sync.WaitGroup
	// for i, url := range categoryURLs {
	// 	wg.Add(1)
	// 	categoryProxy := proxies[i%len(proxies)]
	// 	fmt.Println("categoryProxy", categoryProxy)

	// 	go func(url string) {
	// 		defer wg.Done()

	// 		allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), append(chromedp.DefaultExecAllocatorOptions[:], chromedp.Flag("headless", false), chromedp.ProxyServer(categoryProxy))...)
	// 		defer cancel()

	// 		tabCtx, cancel := chromedp.NewContext(allocCtx)
	// 		defer cancel()
	// 		var products []Product
	// 		err := chromedp.Run(tabCtx,
	// 			network.Enable(),
	// 			network.SetExtraHTTPHeaders(network.Headers(headers)),
	// 			chromedp.Navigate(url),
	// 			chromedp.Sleep(time.Duration(rand.Intn(1000)+1000)*time.Millisecond),
	// 			chromedp.ActionFunc(func(ctx context.Context) error {
	// 				var iframeExists bool
	// 				chromedp.Evaluate(`document.querySelector("iframe[src*='geo.captcha-delivery.com']") !== null`, &iframeExists).Do(ctx)
	// 				if iframeExists {
	// 					fmt.Println("Captcha detected, reloading page...")
	// 					err := chromedp.Reload().Do(ctx)
	// 					if err != nil {
	// 						log.Println("Error reloading page:", err)
	// 						return nil
	// 					}
	// 					log.Println("Page reloaded due to captcha, continuing scraping...")
	// 				} else {
	// 					fmt.Println("No captcha detected, proceeding with scraping...")
	// 				}

	// 				var jsonData string
	// 				err := chromedp.Evaluate(`document.querySelector("script[type='application/ld+json']").innerText`, &jsonData).Do(ctx)
	// 				if err != nil {
	// 					return fmt.Errorf("failed to extract JSON data: %w", err)
	// 				}

	// 				var rawData struct {
	// 					ItemListElement []struct {
	// 						Url    string `json:"url"`
	// 						Name   string `json:"name"`
	// 						Offers struct {
	// 							Price         string `json:"price"`
	// 							PriceCurrency string `json:"priceCurrency"`
	// 						} `json:"offers"`
	// 						Brand struct {
	// 							Name string `json:"name"`
	// 						} `json:"brand"`
	// 						Position int `json:"position"`
	// 					} `json:"itemListElement"`
	// 				}

	// 				err = json.Unmarshal([]byte(jsonData), &rawData)
	// 				if err != nil {
	// 					return fmt.Errorf("failed to parse JSON data: %w", err)
	// 				}

	// 				for _, item := range rawData.ItemListElement {
	// 					var productId string
	// 					re := regexp.MustCompile(`/listing/(\d+)`)
	// 					matches := re.FindStringSubmatch(item.Url)
	// 					if len(matches) > 1 {
	// 						productId = matches[1]
	// 					}
	// 					products = append(products, Product{
	// 						ProductID:    productId,
	// 						ProductName:  item.Name,
	// 						ProductPrice: item.Offers.Price,
	// 					})
	// 				}
	// 				return nil
	// 			}),
	// 		)

	// 		if err != nil {
	// 			log.Println("Error while scraping category:", url, err)
	// 		} else {
	// 			fmt.Println("Successfully scraped category:", url)
	// 			fmt.Println("Products:")
	// 			for _, product := range products {
	// 				fmt.Printf("ID: %s, Name: %s, Price: %s\n", product.ProductID, product.ProductName, product.ProductPrice)
	// 			}
	// 		}
	// 	}(url)
	// }

	// wg.Wait()
	fmt.Println("Success Scraping Data")
}
