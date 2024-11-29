package consumer

import (
	"ecommerce-scraping-analytics/internal/config"
	"ecommerce-scraping-analytics/internal/controllers"
	"ecommerce-scraping-analytics/internal/entity"
	"encoding/json"
	"fmt"
)

type MainControllerConsumer struct {
	Controller *controllers.MainController
}

func (mainController MainControllerConsumer) ConsumeSellerProductResponse(rabbitMQConfig *config.RabbitMqConfig) {
	queueName := "ProductSellerResponseQueue"
	q, err := rabbitMQConfig.Channel.QueueDeclare(
		queueName, // queue name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		fmt.Printf("Failed to declare a queue: %v", err)
		return
	}

	msgs, err := rabbitMQConfig.Channel.Consume(
		q.Name,             // Queue name
		"ConsumerListener", // Consumer tag
		true,               // Auto-acknowledge
		false,              // Exclusive
		false,              // No-local
		false,              // No-wait
		nil,                // Args
	)
	if err != nil {
		fmt.Printf("Queue '%s' not available. Retrying in 5 seconds... Error: %v", queueName, err)
		return
	}

	fmt.Println("Consumer started for queue:", queueName)

	for msg := range msgs {
		var responseMessage map[string]interface{}
		err := json.Unmarshal(msg.Body, &responseMessage)
		if err != nil {
			fmt.Printf("Failed to unmarshal response: %v\n", err)
			continue
		}

		if responseMessage["message"] == "responseSuccess" {
			data, ok := responseMessage["data"].([]interface{})
			if !ok {
				fmt.Println("Invalid data format")
				continue
			}

			var responseData []entity.CategoryProducts
			dataBytes, err := json.Marshal(data)
			if err != nil {
				fmt.Printf("Failed to marshal data: %v\n", err)
				continue
			}
			err = json.Unmarshal(dataBytes, &responseData)
			if err != nil {
				fmt.Printf("Failed to unmarshal category products: %v\n", err)
				continue
			}
			mainController.Controller.ResponseChannel <- responseData
		}
	}
}
