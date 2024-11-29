package controllers

import (
	"ecommerce-scraping-analytics/internal/config"
	"ecommerce-scraping-analytics/internal/entity"
	"ecommerce-scraping-analytics/internal/model/response"
	"ecommerce-scraping-analytics/internal/rabbitmq/producer"
	"net/http"

	"github.com/gin-gonic/gin"
)

type MainController struct {
	LogicController *LogicController
	Rabbitmq        *config.RabbitMqConfig
	Producer        *producer.MainControllerProducer
	ResponseChannel chan []entity.CategoryProducts
}

func NewMainController(logic *LogicController, rabbitMq *config.RabbitMqConfig, producer *producer.MainControllerProducer) *MainController {
	return &MainController{
		LogicController: logic,
		Rabbitmq:        rabbitMq,
		Producer:        producer,
		ResponseChannel: make(chan []entity.CategoryProducts),
	}
}

func (mainController *MainController) GetSellerProductsBySeller(c *gin.Context) {
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
		"GetSellerProduct Queue", // queue name
		true,                     // durable
		false,                    // auto delete
		false,                    // exclusive
		false,                    // no wait
		nil,                      // arguments
	)
	if err != nil {
		result := &response.Response[interface{}]{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		}
		response.NewResponse(c.Writer, result)
	}
	mainController.Producer.CreateMessageGetSellerProducts(rabbitMqchannel, seller)

	responseData := <-mainController.ResponseChannel
	result := &response.Response[interface{}]{
		Code:    http.StatusOK,
		Message: "Success",
		Data:    responseData,
	}
	response.NewResponse(c.Writer, result)
}
