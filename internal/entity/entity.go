package entity

type Product struct {
	ProductID          string `json:"product_id"`
	ProductTitle       string `json:"product_title"`
	ProductURL         string `json:"product_url"`
	ProductPrice       string `json:"product_price"`
	ProductStock       string `json:"product_stock,omitempty"`
	ProductSold        string `json:"product_sold,omitempty"`
	ProductRating      string `json:"product_rating,omitempty"`
	ProductRatingCount string `json:"product_rating_count,omitempty"`
}

type CategoryProducts struct {
	CategoryID   string    `json:"category_id"`
	CategoryName string    `json:"category_name"`
	Products     []Product `json:"products"`
}
