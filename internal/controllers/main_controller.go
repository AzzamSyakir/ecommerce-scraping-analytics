package controllers

import (
	"ecommerce-scraping-analytics/internal/config"
	"ecommerce-scraping-analytics/internal/model/response"
	"ecommerce-scraping-analytics/internal/rabbitmq/producer"
	"net/http"

	"github.com/gin-gonic/gin"
)

type MainController struct {
	LogicController *LogicController
	Rabbitmq        *config.RabbitMqConfig
	Producer        *producer.MainControllerProducer
}

func NewMainController(logic *LogicController, rabbitMq *config.RabbitMqConfig, producer *producer.MainControllerProducer) *MainController {
	mainController := &MainController{
		LogicController: logic,
		Rabbitmq:        rabbitMq,
		Producer:        producer,
	}
	return mainController
}

func (mainController *MainController) GetPopularProducts(c *gin.Context) {
	seller := c.Param("seller")
	RabbitMQConnection := mainController.Rabbitmq.Connection
	rabbitMqchannel, err := RabbitMQConnection.Channel()
	if err != nil {
		result := &response.Response[interface{}]{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		}
		response.NewResponse(c.Writer, result)
	}

	_, err = rabbitMqchannel.QueueDeclare(
		"GetPopularProduct Queue", // queue name
		true,                      // durable
		false,                     // auto delete
		false,                     // exclusive
		false,                     // no wait
		nil,                       // arguments
	)
	if err != nil {
		result := &response.Response[interface{}]{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		}
		response.NewResponse(c.Writer, result)
	}
	mainController.Producer.CreateMessageGetPopularProducts(rabbitMqchannel, seller)
}
