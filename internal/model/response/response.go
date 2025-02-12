package response

import (
	"ecommerce-scraping-analytics/internal/entity"
	"encoding/json"
	"net/http"
)

type SellerProductResponse struct {
	Categories        []entity.CategoryProducts `json:"categories"`
	ItemsSold         int                       `json:"total_item_sold"`
	ProductsSoldCount int                       `json:"total_product_listing_sold"`
}
type Response[T any] struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Data    T      `json:"data,omitempty"`
}

func NewResponse[T any](w http.ResponseWriter, result *Response[T]) {
	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(result.Code)
	encodeErr := json.NewEncoder(w).Encode(result)
	if encodeErr != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
