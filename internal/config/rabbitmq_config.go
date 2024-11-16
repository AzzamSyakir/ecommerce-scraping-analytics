package config

import (
	"os"

	"github.com/streadway/amqp"
)

type RabbitMqConfig struct {
	Connection *amqp.Connection
	Channel    *amqp.Channel
}

func NewRabbitMqConfig() *RabbitMqConfig {
	connection, err := RabbitMQConnection()
	if err != nil {
		panic("failed to establish RabbitMQ connection: " + err.Error())
	}
	channel, err := RabbitMqChannel(connection)
	if err != nil {
		panic("failed to establish RabbitMQ channel: " + err.Error())
	}
	rabbitMqConfig := &RabbitMqConfig{
		Connection: connection,
		Channel:    channel,
	}
	return rabbitMqConfig
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
