package producer

import (
	"fmt"

	"github.com/streadway/amqp"
)

type MainControllerProducer struct{}

func CreateNewMainControllerProducer() *MainControllerProducer {
	mainControllerProducer := &MainControllerProducer{}
	return mainControllerProducer
}

func (*MainControllerProducer) CreateMessageToScrapingController(channelRabbitMQ *amqp.Channel) error {
	queueName := "ProductCategoyTrends Queue"
	msg := "Start Scraping"
	message := amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte(msg),
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
	return nil
}
