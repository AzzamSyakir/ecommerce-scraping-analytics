package controllers

import (
	"etsy-trend-analytics/internal/config"
	"etsy-trend-analytics/internal/model/response"
	"etsy-trend-analytics/internal/rabbitmq/producer"
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

func (mainController *MainController) ProductCategoryTrendsMainController(c *gin.Context) {
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
		"ProductCategoyTrends Queue", // queue name
		true,                         // durable
		false,                        // auto delete
		false,                        // exclusive
		false,                        // no wait
		nil,                          // arguments
	)
	if err != nil {
		result := &response.Response[interface{}]{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		}
		response.NewResponse(c.Writer, result)
	}
	mainController.Producer.CreateMessageToScrapingController(rabbitMqchannel)
}
