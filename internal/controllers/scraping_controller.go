package controllers

import (
	"fmt"
	"log"

	"github.com/gocolly/colly"
)

type ScrapingController struct{}

func NewScrapingController() *ScrapingController {
	scrapingController := &ScrapingController{}
	return scrapingController
}

func (scrapingcontroller *ScrapingController) ProductCategoryTrendsScrapingController() {
	fmt.Println("Success Scraping Data")
	var categoryURLs []string
	c := colly.NewCollector()
	url := "https://www.etsy.com/"
	c.OnHTML("div[role='menu'] a", func(e *colly.HTMLElement) {
		categoryURL := "https://www.etsy.com" + e.Attr("href")
		categoryURLs = append(categoryURLs, categoryURL)

	})

	err := c.Visit(url)
	if err != nil {
		log.Fatal(err)
	}
	c.Wait()
	for _, categoryURL := range categoryURLs {
		err := c.Visit(categoryURL)
		if err != nil {
			log.Println("Error visiting category:", err)
		}
	}
}
