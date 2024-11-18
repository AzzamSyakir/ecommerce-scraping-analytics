package controllers

import (
	"context"
	"fmt"
	"log"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
)

type ScrapingController struct{}

func NewScrapingController() *ScrapingController {
	scrapingController := &ScrapingController{}
	return scrapingController
}

func (scrapingcontroller *ScrapingController) ProductCategoryTrendsScrapingController() {
	ctx, cancel := chromedp.NewExecAllocator(context.Background(), append(chromedp.DefaultExecAllocatorOptions[:], chromedp.Flag("headless", false))...)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()
	var categoryURLs []string
	url := "https://www.etsy.com/"

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		// chromedp.WaitVisible("div[role='menu']", chromedp.ByQuery),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var nodes []*cdp.Node
			err := chromedp.Nodes("div[role='menu'] a", &nodes, chromedp.ByQueryAll).Do(ctx)
			fmt.Println("tes")
			if err != nil {
				return fmt.Errorf("failed to query nodes: %w", err)
			}
			for _, node := range nodes {
				var href string
				err := chromedp.AttributeValue(node, "href", &href, nil).Do(ctx)
				if err != nil {
					log.Println("Failed to get href for node:", node, "Error:", err)
					continue
				}
				categoryURLs = append(categoryURLs, href)
			}

			return nil
		}),
	)

	if err != nil {
		log.Fatal("Error while performing the automation logic:", err)
	}

	fmt.Println(categoryURLs)
	fmt.Println("Success Scraping Data")
}
