package entity

type Product struct {
	ProductID     string  `json:"product_id"`
	ProductTitle  string  `json:"product_title"`
	ProductURL    string  `json:"product_url"`
	ProductPrice  string  `json:"product_price"`
	ProductStock  string  `json:"product_stock,omitempty"`
	ProductSold   int     `json:"product_sold,omitempty"`
	ProductRating float64 `json:"product_rating,omitempty"`
}

type CategoryProducts struct {
	CategoryID   string    `json:"category_id"`
	CategoryName string    `json:"category_name"`
	Products     []Product `json:"products"`
}
