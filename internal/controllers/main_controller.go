package controllers

import (
	"ecommerce-scraping-analytics/internal/config"
	"ecommerce-scraping-analytics/internal/rabbitmq/producer"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response[T any] struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Data    T      `json:"data,omitempty"`
}

type MainController struct {
	LogicController *LogicController
	Rabbitmq        *config.RabbitMqConfig
	Producer        *producer.MainControllerProducer
	ResponseChannel chan Response[interface{}]
}

func NewMainController(logic *LogicController, rabbitMq *config.RabbitMqConfig, producer *producer.MainControllerProducer) *MainController {
	return &MainController{
		LogicController: logic,
		Rabbitmq:        rabbitMq,
		Producer:        producer,
		ResponseChannel: make(chan Response[interface{}], 1),
	}
}

func (mainController *MainController) GetSellerProductsBySeller(c *gin.Context) {
	seller := c.Param("seller")
	RabbitMQConnection := mainController.Rabbitmq.Connection
	rabbitMqChannel, err := RabbitMQConnection.Channel()
	if err != nil {
		result := &Response[interface{}]{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		}
		c.JSON(result.Code, result)
		return
	}
	defer rabbitMqChannel.Close()

	_, err = rabbitMqChannel.QueueDeclare(
		"GetSellerProduct Queue",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		result := &Response[interface{}]{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		}
		c.JSON(result.Code, result)
		return
	}

	mainController.Producer.CreateMessageGetSellerProducts(rabbitMqChannel, seller)
	responseData := <-mainController.ResponseChannel
	var zeroResponse Response[interface{}]
	if responseData != zeroResponse {
		c.JSON(responseData.Code, responseData)
	} else {
		result := &Response[interface{}]{
			Code:    http.StatusBadRequest,
			Message: "Failed to retrieve products, cannot get response from message rabbitMq",
		}
		c.JSON(result.Code, result)
		return
	}
}
