package response

import "ecommerce-scraping-analytics/internal/entity"

type SellerProductResponse struct {
	Categories        []entity.CategoryProducts `json:"categories"`
	ItemsSold         int                       `json:"total_item_sold"`
	ProductsSoldCount int                       `json:"total_product_listing_sold"`
}
