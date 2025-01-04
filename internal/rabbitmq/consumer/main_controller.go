package consumer

import (
	"ecommerce-scraping-analytics/internal/config"
	"ecommerce-scraping-analytics/internal/controllers"
	"ecommerce-scraping-analytics/internal/model/response"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

type MainControllerConsumer struct {
	Controller *controllers.MainController
}
type RabbitMQPayload struct {
	Message string                         `json:"message"`
	Data    response.SellerProductResponse `json:"data"`
}

func (mainController MainControllerConsumer) ConsumeSellerProductResponse(rabbitMQConfig *config.RabbitMqConfig) {
	queueName := "ProductSellerResponseQueue"
	q, err := rabbitMQConfig.Channel.QueueDeclare(
		queueName,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		fmt.Printf("Failed to declare a queue: %v\n", err)
		return
	}

	msgs, err := rabbitMQConfig.Channel.Consume(
		q.Name,
		"ConsumerListener",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		fmt.Printf("Queue '%s' not available. Retrying in 5 seconds... Error: %v\n", queueName, err)
		return
	}

	for msg := range msgs {
		var payload RabbitMQPayload
		// Parse JSON message
		err := json.Unmarshal(msg.Body, &payload)
		if err != nil {
			log.Fatal("Failed to unmarshal message: ", err)
		}

		// Handle error response
		if strings.HasPrefix(payload.Message, "responseError") {
			errorMessage := strings.TrimPrefix(payload.Message, "responseError")
			errorMessage = strings.TrimSpace(errorMessage)

			if errorMessage == "" {
				mainController.Controller.ResponseChannel <- controllers.Response[interface{}]{
					Code:    500,
					Message: "Error message is empty after 'responseError'",
					Data:    payload.Data,
				}
				continue
			}

			mainController.Controller.ResponseChannel <- controllers.Response[interface{}]{
				Code:    400,
				Message: fmt.Sprintf("Error occurred: %s", errorMessage),
				Data:    payload.Data,
			}
			continue
		}

		// Handle success response
		if payload.Message == "responseSuccess" {
			dataBytes, err := json.Marshal(payload.Data)
			if err != nil {
				fmt.Printf("Failed to marshal response data: %v\n", err)
				continue
			}

			var responseData *response.SellerProductResponse
			err = json.Unmarshal(dataBytes, &responseData)
			if err != nil {
				fmt.Printf("Failed to unmarshal category products: %v\n", err)
				continue
			}

			mainController.Controller.ResponseChannel <- controllers.Response[interface{}]{
				Code:    200,
				Message: "Success",
				Data:    responseData,
			}
		} else {
			mainController.Controller.ResponseChannel <- controllers.Response[interface{}]{
				Code:    400,
				Message: "Unknown message type",
				Data:    nil,
			}
		}
	}
}
