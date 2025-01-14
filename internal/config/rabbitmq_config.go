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
	rabbitMqHost := os.Getenv("RABBITMQ_HOST")
	rabbitMqUser := os.Getenv("RABBITMQ_USER")
	rabbitMqPass := os.Getenv("RABBITMQ_PASS")
	rabbitMqPort := os.Getenv("RABBITMQ_PORT")
	amqpServerURL := fmt.Sprintf("amqp://%s:%s@%s:%s", rabbitMqUser, rabbitMqPass, rabbitMqHost, rabbitMqPort)
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
