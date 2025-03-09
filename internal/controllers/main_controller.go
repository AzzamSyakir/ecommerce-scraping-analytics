package controllers

import (
	"ecommerce-scraping-analytics/internal/config"
	"ecommerce-scraping-analytics/internal/rabbitmq/producer"
	"net/http"

	"ecommerce-scraping-analytics/internal/model/response"

	"github.com/gorilla/mux"
)

type MainController struct {
	LogicController *LogicController
	Rabbitmq        *config.RabbitMqConfig
	Producer        *producer.MainControllerProducer
	ResponseChannel chan response.Response[interface{}]
}

func NewMainController(logic *LogicController, rabbitMq *config.RabbitMqConfig, producer *producer.MainControllerProducer) *MainController {
	return &MainController{
		LogicController: logic,
		Rabbitmq:        rabbitMq,
		Producer:        producer,
		ResponseChannel: make(chan response.Response[interface{}], 1),
	}
}

func (mainController *MainController) GetAllSellerProducts(writer http.ResponseWriter, reader *http.Request) {
	vars := mux.Vars(reader)
	seller := vars["seller"]
	mainController.Producer.CreateMessageGetAllSellerProducts(mainController.Rabbitmq.Channel, seller)
	responseData := <-mainController.ResponseChannel
	var zeroResponse response.Response[map[string]any]
	if responseData.Code != zeroResponse.Code {
		response.NewResponse(writer, &responseData)
	} else {
		result := &response.Response[map[string]any]{
			Code:    http.StatusBadRequest,
			Message: "Failed to retrieve products, cannot get response from message rabbitMq",
		}
		response.NewResponse(writer, result)
		return
	}
}

func (mainController *MainController) GetSoldSellerProducts(writer http.ResponseWriter, reader *http.Request) {
	vars := mux.Vars(reader)
	seller := vars["seller"]
	mainController.Producer.CreateMessageGetSoldSellerProducts(mainController.Rabbitmq.Channel, seller)
	responseData := <-mainController.ResponseChannel
	var zeroResponse response.Response[map[string]any]
	if responseData.Code != zeroResponse.Code {
		response.NewResponse(writer, &responseData)
	} else {
		result := &response.Response[map[string]any]{
			Code:    http.StatusBadRequest,
			Message: "Failed to retrieve products, cannot get response from message rabbitMq",
		}
		response.NewResponse(writer, result)
		return
	}
}
