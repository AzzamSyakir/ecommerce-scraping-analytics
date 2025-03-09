package consumer

import (
	"ecommerce-scraping-analytics/internal/config"
	"ecommerce-scraping-analytics/internal/controllers"
)

type ConsumerEntrypoint struct {
	MainConsumer *MainControllerConsumer
	// LogicConsumer    *LogicControllerConsumer
	ScrapingConsumer *ScrapingControllerConsumer
	RabbitMQ         *config.RabbitMqConfig
}

func NewConsumerEntrypointInit(rabbitMQConfig *config.RabbitMqConfig, mainController *controllers.MainController, scrapingController *controllers.ScrapingController, envConfig *config.EnvConfig) *ConsumerEntrypoint {
	return &ConsumerEntrypoint{
		MainConsumer: &MainControllerConsumer{Controller: mainController, Env: envConfig},
		// LogicConsumer:    &LogicControllerConsumer{Controller: &logic.LogicController{}},
		ScrapingConsumer: &ScrapingControllerConsumer{Controller: scrapingController, Env: envConfig},
		RabbitMQ:         rabbitMQConfig,
	}
}

func (consumerEntrypoint *ConsumerEntrypoint) ConsumerEntrypointStart() {
	go consumerEntrypoint.ScrapingConsumer.ConsumeMessageAllSellerProduct(consumerEntrypoint.RabbitMQ)
	go consumerEntrypoint.ScrapingConsumer.ConsumeMessageSoldSellerProduct(consumerEntrypoint.RabbitMQ)
	go consumerEntrypoint.MainConsumer.ConsumeSellerProductResponse(consumerEntrypoint.RabbitMQ)
}
