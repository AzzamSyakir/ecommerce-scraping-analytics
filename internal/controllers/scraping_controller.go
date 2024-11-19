package controllers

import (
	"context"
	"fmt"
	"log"
	"math/rand"
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

func (scrapingcontroller *ScrapingController) ProductCategoryTrendsScrapingController() {
	headers := map[string]interface{}{
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
		"Accept-Encoding":           "gzip, deflate, br",
		"Accept-Language":           "en-US,en;q=0.5",
		"Cache-Control":             "max-age=0",
		"Connection":                "keep-alive",
		"Upgrade-Insecure-Requests": "1",
		"User-Agent":                "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36",
		"Referer":                   "https://www.etsy.com",
		"Sec-CH-UA":                 "\"Google Chrome\";v=\"114\", \"Chromium\";v=\"114\", \"Not_A Brand\";v=\"99\"",
		"Sec-CH-UA-Mobile":          "?0",
		"Sec-CH-UA-Platform":        "\"Linux\"",
	}

	ctx, cancel := chromedp.NewExecAllocator(context.Background(), append(chromedp.DefaultExecAllocatorOptions[:], chromedp.Flag("headless", false))...)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()

	var categoryURLs []string
	url := "https://www.etsy.com/"

	err := chromedp.Run(ctx,
		network.Enable(),
		network.SetExtraHTTPHeaders(network.Headers(headers)),
		chromedp.Navigate(url),
		chromedp.Sleep(time.Duration(rand.Intn(1000)+1000)*time.Millisecond),
		chromedp.Reload(),
		chromedp.Sleep(time.Duration(rand.Intn(5000)+1000)*time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error {
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
						log.Printf("Extracted href: %s\n", href)
						ch <- href
					}
				}(node)
			}

			for i := 0; i < len(nodes); i++ {
				href := <-ch
				if href != "" {
					categoryURLs = append(categoryURLs, href)
				}
			}

			return nil
		}),
	)

	if err != nil {
		log.Fatal("Error while performing the automation logic:", err)
	}
	fmt.Println("Success Scraping Data")
}
