package config

import (
	"fmt"
	"os"

	"github.com/streadway/amqp"
)

type RabbitMqConfig struct {
	Connection *amqp.Connection
	Channel    *amqp.Channel
}

func NewRabbitMqConfig() (*RabbitMqConfig, error) {
	connection, err := RabbitMQConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to establish RabbitMQ connection: %w", err)
	}
	channel, err := RabbitMqChannel(connection)
	if err != nil {
		return nil, fmt.Errorf("failed to establish RabbitMQ channel: %w", err)
	}
	rabbitMqConfig := &RabbitMqConfig{
		Connection: connection,
		Channel:    channel,
	}
	return rabbitMqConfig, nil
}

func RabbitMQConnection() (*amqp.Connection, error) {
	amqpServerURL := os.Getenv("AMQP_SERVER_URL")

	connectRabbitMQ, err := amqp.Dial(amqpServerURL)
	if err != nil {
		return nil, err
	}
	return connectRabbitMQ, nil
}

func RabbitMqChannel(connection *amqp.Connection) (*amqp.Channel, error) {

	channelRabbitMQ, err := connection.Channel()
	if err != nil {
		return nil, err
	}
	return channelRabbitMQ, nil
}

func (*RabbitMqConfig) CreateMessage(channelRabbitMQ *amqp.Channel, msg string, queueName string) error {
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

func (*RabbitMqConfig) ConsumeMessage(channelRabbitMQ *amqp.Channel, msg any, queueName string) error {
	_, err := channelRabbitMQ.Consume(
		queueName, // queue name
		"",        // consumer
		true,      // auto-ack
		false,     // exclusive
		false,     // no local
		false,     // no wait
		nil,       // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to publish message to queue: %w", err)
	}
	return nil
}
