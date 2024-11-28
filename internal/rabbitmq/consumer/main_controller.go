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

func (scrapingControllerConsumer *ScrapingControllerConsumer) ConsumedataProductSellerScrape(rabbitMQConfig *config.RabbitMqConfig) ([]entity.CategoryProducts, error) {
	queueName := "PublishSellerProduct Queue"
	q, err := rabbitMQConfig.Channel.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		fmt.Printf("Failed to declare a queue:%v", err)
	}
	expectedMessage := "Display Data"
	msgs, err := rabbitMQConfig.Channel.Consume(
		q.Name,              // Queue name
		"Scraping Consumer", // Consumer tag
		true,                // Auto-acknowledge
		false,               // Exclusive
		false,               // No-local
		false,               // No-wait
		nil,                 // Args
	)
	if err != nil {
		fmt.Printf("Queue '%s' not available. Retrying in 5 seconds... Error: %v", queueName, err)
	}

	fmt.Printf("Consumer started for queue: %s", queueName)

	for msg := range msgs {
		var message map[string]interface{}
		json.Unmarshal(msg.Body, &message)
		if message["message"] == expectedMessage {
			data, ok := message["data"].([]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid data format in message")
			}
			var responseData []entity.CategoryProducts
			dataBytes, err := json.Marshal(data)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal data: %v", err)
			}
			err = json.Unmarshal(dataBytes, &responseData)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal category products: %v", err)
			}
			return responseData, nil
		} else {
			fmt.Printf("Message '%v' does not match expected message '%s'. Ignoring...", message, expectedMessage)
		}
	}
	return nil, fmt.Errorf("no valid message received")
}
