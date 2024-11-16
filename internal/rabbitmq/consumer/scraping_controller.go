package consumer

import (
	"etsy-trend-analytics/internal/controllers"
	"fmt"
	"strings"

	"github.com/streadway/amqp"
)

type ScrapingControllerConsumer struct {
	Controller *controllers.ScrapingController
}

func (scrapingControllerConsumer *ScrapingControllerConsumer) ConsumeMessageFromMainController(channelRabbitMQ *amqp.Channel) error {
	queueName := "ProductCategoyTrends Queue"
	msgs, err := channelRabbitMQ.Consume(
		queueName, // Queue name
		"",        // Consumer tag
		true,      // Auto-acknowledge
		false,     // Exclusive
		false,     // No-local
		false,     // No-wait
		nil,       // Args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %v", err)
	}

	for msg := range msgs {
		message := string(msg.Body)
		if strings.Contains(message, "Start Scraping") {
			scrapingControllerConsumer.Controller.ProductCategoryTrendsScrapingController()
		}
	}
	return nil
}
