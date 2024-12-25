package entity

type Product struct {
	ProductID          string `json:"product_id"`
	ProductTitle       string `json:"product_title"`
	ProductURL         string `json:"product_url"`
	ProductPrice       string `json:"product_price"`
	ProductStock       string `json:"product_stock"`
	ProductSold        string `json:"product_sold"`
	ProductRating      string `json:"product_rating"`
	ProductRatingCount string `json:"product_rating_count"`
}

type CategoryProducts struct {
	CategoryID   string    `json:"category_id"`
	CategoryName string    `json:"category_name"`
	Products     []Product `json:"products"`
}
