package producer

import (
	"ecommerce-scraping-analytics/internal/config"
	"encoding/json"
	"fmt"

	"github.com/streadway/amqp"
)

type MainControllerProducer struct {
	Env *config.EnvConfig
}

func CreateNewMainControllerProducer(envConfig *config.EnvConfig) *MainControllerProducer {
	mainControllerProducer := &MainControllerProducer{
		Env: envConfig,
	}
	return mainControllerProducer
}

func (mainControllerProducer *MainControllerProducer) CreateMessageGetAllSellerProducts(channelRabbitMQ *amqp.Channel, seller string) error {
	queueName := mainControllerProducer.Env.RabbitMq.Queues[1]
	payload := map[string]any{
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

func (mainControllerProducer *MainControllerProducer) CreateMessageGetSoldSellerProducts(channelRabbitMQ *amqp.Channel, seller string) error {
	queueName := mainControllerProducer.Env.RabbitMq.Queues[2]
	payload := map[string]any{
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
