package producer

import (
	"encoding/json"
	"fmt"

	"github.com/streadway/amqp"
)

type MainControllerProducer struct{}

func CreateNewMainControllerProducer() *MainControllerProducer {
	mainControllerProducer := &MainControllerProducer{}
	return mainControllerProducer
}

func (*MainControllerProducer) CreateMessageGetAllSellerProducts(channelRabbitMQ *amqp.Channel, seller string) error {
	queueName := "GetAllSellerProduct Queue"
	payload := map[string]interface{}{
		"message": "Start Scraping",
		"seller":  seller,
		"channel": channelRabbitMQ,
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
	return nil
}

func (*MainControllerProducer) CreateMessageGetSoldSellerProducts(channelRabbitMQ *amqp.Channel, seller string) error {
	queueName := "GetSoldSellerProduct Queue"
	payload := map[string]interface{}{
		"message": "Start Scraping",
		"seller":  seller,
		"channel": channelRabbitMQ,
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
	return nil
}
