package controller

import (
	"etsy-trend-analytics/internal/config"
	"etsy-trend-analytics/internal/controller/logic"
	"etsy-trend-analytics/internal/model/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

type MainController struct {
	LogicController *logic.LogicController
	Rabbitmq        *config.RabbitMqConfig
}

func NewMainController(logic *logic.LogicController, rabbitMq *config.RabbitMqConfig) *MainController {
	return &MainController{LogicController: logic, Rabbitmq: rabbitMq}
}

func (mainController *MainController) ProductCategoryTrends(c *gin.Context) {
	rabbitMqInit, err := config.NewRabbitMqConfig()

	if err != nil {
		result := &response.Response[interface{}]{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		}
		response.NewResponse(c.Writer, result)
	}

	defer rabbitMqInit.Connection.Close()

	rabbitMqchannel := rabbitMqInit.Channel
	defer rabbitMqchannel.Close()

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
	message := "Start Scraping"
	rabbitMqInit.CreateMessage(rabbitMqchannel, message, "ProductCategoyTrends Queue")
}
