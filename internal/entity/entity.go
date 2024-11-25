package entity

import (
	"time"

	"github.com/google/uuid"
)

type GetPopularProducts struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	CategoryName string    `gorm:"type:varchar(100);not null" json:"category_name"`
	SalesVolume  int       `gorm:"not null" json:"sales_volume"`
	TrendStatus  string    `gorm:"type:varchar(50);not null" json:"trend_status"`
	AnalysisDate time.Time `gorm:"type:date;not null" json:"analysis_date"`
	PriceAverage float64   `gorm:"type:numeric(10,2)" json:"price_average"`
}
