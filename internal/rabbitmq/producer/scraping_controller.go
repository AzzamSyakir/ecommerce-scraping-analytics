package producer

import (
	"ecommerce-scraping-analytics/internal/entity"
	"encoding/json"
	"fmt"

	"github.com/streadway/amqp"
)

type ScrapingControllerProducer struct{}

func CreateNewScrapingControllerProducer() *ScrapingControllerProducer {
	mainControllerProducer := &ScrapingControllerProducer{}
	return mainControllerProducer
}

func (*ScrapingControllerProducer) PublishScrapingData(msg string, channelRabbitMQ *amqp.Channel, data []entity.CategoryProducts) error {
	queueName := "ProductSellerResponseQueue"
	payload := map[string]interface{}{
		"message": msg,
		"data":    data,
	}

	messageBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal message body: %w", err)
	}
	message := amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte(messageBody),
	}
	if err := channelRabbitMQ.Publish(
		"",        // exchange
		queueName, // queue name
		false,     // mandatory
		false,     // immediate
		message,   // message to publish
	); err != nil {
		return fmt.Errorf("failed to publish message to queue: %w", err)
	}
	fmt.Println("publish scrape data finished")
	return nil
}
