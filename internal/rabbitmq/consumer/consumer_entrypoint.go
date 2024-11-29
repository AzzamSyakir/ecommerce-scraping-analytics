package consumer

import (
	"ecommerce-scraping-analytics/internal/config"
	"ecommerce-scraping-analytics/internal/controllers"
	"ecommerce-scraping-analytics/internal/entity"
)

type ConsumerEntrypoint struct {
	MainConsumer *MainControllerConsumer
	// LogicConsumer    *LogicControllerConsumer
	ScrapingConsumer *ScrapingControllerConsumer
	RabbitMQ         *config.RabbitMqConfig
}

func NewConsumerEntrypointInit(rabbitMQConfig *config.RabbitMqConfig, mainController *controllers.MainController, scrapingController *controllers.ScrapingController) *ConsumerEntrypoint {
	return &ConsumerEntrypoint{
		MainConsumer: &MainControllerConsumer{Controller: mainController},
		// LogicConsumer:    &LogicControllerConsumer{Controller: &logic.LogicController{}},
		ScrapingConsumer: &ScrapingControllerConsumer{Controller: scrapingController},
		RabbitMQ:         rabbitMQConfig,
	}
}

func (consumerEntrypoint *ConsumerEntrypoint) ConsumerEntrypointStart() {
	responseChannel := make(chan []entity.CategoryProducts)
	go consumerEntrypoint.ScrapingConsumer.ConsumeMessageSellerProduct(consumerEntrypoint.RabbitMQ)
	go consumerEntrypoint.MainConsumer.ConsumeSellerProductResponse(consumerEntrypoint.RabbitMQ, responseChannel)
}
