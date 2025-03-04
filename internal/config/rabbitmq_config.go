package config

import (
	"fmt"

	"github.com/streadway/amqp"
)

type RabbitMqConfig struct {
	Connection *amqp.Connection
	Channel    *amqp.Channel
	Queue      []*amqp.Queue
	Env        *EnvConfig
}

func NewRabbitMqConfig(env *EnvConfig) *RabbitMqConfig {
	connection, err := RabbitMQConnection(env)
	if err != nil {
		panic("failed to establish RabbitMQ connection: " + err.Error())
	}
	channel, err := RabbitMqChannel(connection)
	if err != nil {
		panic("failed to establish RabbitMQ channel: " + err.Error())
	}
	queue, err := RabbitMqQueue(channel, env)
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

func RabbitMQConnection(env *EnvConfig) (*amqp.Connection, error) {
	rabbitMqHost := env.RabbitMq.Host
	rabbitMqUser := env.RabbitMq.User
	rabbitMqPass := env.RabbitMq.Password
	rabbitMqPort := env.RabbitMq.Port
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

func RabbitMqQueue(channel *amqp.Channel, env *EnvConfig) ([]*amqp.Queue, error) {
	var declaredQueues []*amqp.Queue
	for _, name := range env.RabbitMq.Queues {
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
