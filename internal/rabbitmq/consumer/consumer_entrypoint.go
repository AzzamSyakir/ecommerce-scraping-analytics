package consumer

import (
	"etsy-trend-analytics/internal/config"
	"etsy-trend-analytics/internal/controllers"
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
	go consumerEntrypoint.ScrapingConsumer.ConsumeMessageProductCategory(consumerEntrypoint.RabbitMQ)
}
