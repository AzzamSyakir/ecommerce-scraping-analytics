package config

import (
	"fmt"
	"os"

	"github.com/streadway/amqp"
)

type RabbitMqConfig struct {
	connection *amqp.Connection
	channel    *amqp.Channel
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
		connection: connection,
		channel:    channel,
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
