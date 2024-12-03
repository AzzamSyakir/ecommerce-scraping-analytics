package entity

type Product struct {
	ProductID    string `json:"product_id"`
	ProductTitle string `json:"product_name"`
	ProductURL   string `json:"product_url"`
	ProductPrice string `json:"price"`
	ProductStock string `json:"product_stock"`
	ProductSold  string `json:"product_sold"`
}

type CategoryProducts struct {
	CategoryID   string    `json:"category_id"`
	CategoryName string    `json:"category_name"`
	Products     []Product `json:"products"`
}
