package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/streadway/amqp"
)

type RabbitMqConfig struct {
	Connection *amqp.Connection
	Channel    *amqp.Channel
	Queue      []*amqp.Queue
	Env        *EnvConfig
}

func NewRabbitMqConfig(env *EnvConfig) *RabbitMqConfig {
	connection, err := RabbitMQConnection()
	if err != nil {
		panic("failed to establish RabbitMQ connection: " + err.Error())
	}
	channel, err := RabbitMqChannel(connection)
	if err != nil {
		panic("failed to establish RabbitMQ channel: " + err.Error())
	}
	queue, err := RabbitMqQueue(channel)
	if err != nil {
		panic("failed to establish RabbitMQ queue: " + err.Error())
	}
	rabbitMqConfig := &RabbitMqConfig{
		Connection: connection,
		Channel:    channel,
		Queue:      queue,
		Env:        env,
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

func RabbitMqQueue(channel *amqp.Channel) ([]*amqp.Queue, error) {
	var declaredQueues []*amqp.Queue
	queueNamesStr := os.Getenv("RABBITMQ_QUEUE_NAMES")
	queueNames := strings.Split(queueNamesStr, ",")
	for _, name := range queueNames {
		rabbitmqQueue, err := channel.QueueDeclare(
			name,
			true,  // Durable
			false, // Auto-delete
			true,  // Exclusive
			false, // No-wait
			nil,   // Argument
		)
		if err != nil {
			return nil, err
		}

		declaredQueues = append(declaredQueues, &rabbitmqQueue)
	}

	return declaredQueues, nil
}
