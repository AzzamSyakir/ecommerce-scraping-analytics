package consumer

import (
	"ecommerce-scraping-analytics/internal/config"
	"ecommerce-scraping-analytics/internal/controllers"
	"ecommerce-scraping-analytics/internal/entity"
	"encoding/json"
	"fmt"
	"strings"
)

type MainControllerConsumer struct {
	Controller *controllers.MainController
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

	fmt.Println("Consumer started for queue:", queueName)

	for msg := range msgs {
		var responseMessage controllers.Response[interface{}]

		// Parse JSON message
		err := json.Unmarshal(msg.Body, &responseMessage)
		if err != nil {
			fmt.Printf("Failed to unmarshal message: %v\n", err)
			continue
		}

		// Handle error response
		if strings.HasPrefix(responseMessage.Message, "responseError") {
			errorMessage, ok := responseMessage.Data.(string)
			if !ok || errorMessage == "" {
				fmt.Println("Invalid or empty error message in response")
				mainController.Controller.ResponseChannel <- controllers.Response[interface{}]{
					Code:    500,
					Message: "Invalid error response",
					Data:    nil,
				}
				continue
			}

			fmt.Printf("Received error message: %s\n", errorMessage)
			mainController.Controller.ResponseChannel <- controllers.Response[interface{}]{
				Code:    500,
				Message: errorMessage,
				Data:    nil,
			}
			continue
		}

		// Handle success response
		if responseMessage.Message == "responseSuccess" {
			dataBytes, err := json.Marshal(responseMessage.Data)
			if err != nil {
				fmt.Printf("Failed to marshal response data: %v\n", err)
				continue
			}

			var responseData []entity.CategoryProducts
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
			fmt.Println("Unknown message type:", responseMessage.Message)
			mainController.Controller.ResponseChannel <- controllers.Response[interface{}]{
				Code:    400,
				Message: "Unknown message type",
				Data:    nil,
			}
		}
	}
}
