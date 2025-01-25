package config

import (
	"os"
)

type AppEnv struct {
	AppHost string
	AppPort string
}

type PostgresEnv struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	DBNeed   string
}
type RabbitMqEnv struct {
	Host     string
	Port     string
	User     string
	Password string
	Queue    string
}

type EnvConfig struct {
	App      *AppEnv
	Db       *PostgresEnv
	RabbitMq *RabbitMqEnv
}

func NewEnvConfig() *EnvConfig {
	envConfig := &EnvConfig{
		App: &AppEnv{
			AppHost: os.Getenv("GATEWAY_APP_HOST"),
			AppPort: os.Getenv("GATEWAY_APP_PORT"),
		},
		Db: &PostgresEnv{
			Host:     os.Getenv("POSTGRES_HOST"),
			Port:     os.Getenv("POSTGRES_PORT"),
			User:     os.Getenv("POSTGRES_USER"),
			Password: os.Getenv("POSTGRES_PASSWORD"),
			Database: os.Getenv("POSTGRES_DB"),
			DBNeed:   os.Getenv("POSTGRES_NEED"),
		},
		RabbitMq: &RabbitMqEnv{
			Host:     os.Getenv("RABBITMQ_HOST"),
			Port:     os.Getenv("RABBITMQ_PORT"),
			User:     os.Getenv("RABBITMQ_USER"),
			Password: os.Getenv("RABBITMQ_PASS"),
			Queue:    os.Getenv("RABBITMQ_QUEUE_NAMES"),
		},
	}
	return envConfig
}
