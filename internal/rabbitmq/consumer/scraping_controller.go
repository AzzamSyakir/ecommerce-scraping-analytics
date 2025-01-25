package consumer

import (
	"ecommerce-scraping-analytics/internal/config"
	"ecommerce-scraping-analytics/internal/controllers"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

type ScrapingControllerConsumer struct {
	Controller *controllers.ScrapingController
}

func (scrapingControllerConsumer *ScrapingControllerConsumer) ConsumeMessageAllSellerProduct(rabbitMQConfig *config.RabbitMqConfig) {
	expectedQueueName := "GetAllSellerProductQueue"
	var queueName string
	for _, name := range rabbitMQConfig.Queue {
		if expectedQueueName == name.Name {
			queueName = name.Name
			break
		}
	}
	expectedMessage := "Start Scraping"
	msgs, err := rabbitMQConfig.Channel.Consume(
		queueName,                    // Queue name
		"allSellerProducts Consumer", // Consumer tag
		true,                         // Auto-acknowledge
		false,                        // Exclusive
		false,                        // No-local
		false,                        // No-wait
		nil,                          // Args
	)
	if err != nil {
		log.Printf("Queue '%s' not available. Retrying in 5 seconds... Error: %v", queueName, err)
	}

	for msg := range msgs {
		messageBody := func() map[string]string { m := make(map[string]string); json.Unmarshal([]byte(msg.Body), &m); return m }()
		if messageBody["message"] == expectedMessage {
			scrapingControllerConsumer.Controller.ScrapeAllSellerProducts(messageBody["seller"])
		} else {
			log.Printf("Message '%s' does not match expected message '%s'. Ignoring...", messageBody, expectedMessage)
		}
	}

	log.Println("Message channel closed, attempting to reconnect...")
}
func (scrapingControllerConsumer *ScrapingControllerConsumer) ConsumeMessageSoldSellerProduct(rabbitMQConfig *config.RabbitMqConfig) {
	expectedQueueName := "GetSoldSellerProductQueue"
	var queueName string
	for _, name := range rabbitMQConfig.Queue {
		if expectedQueueName == name.Name {
			queueName = name.Name
			break
		}
	}
	expectedMessage := "Start Scraping"
	msgs, err := rabbitMQConfig.Channel.Consume(
		queueName,                    // Queue name
		"SoldSellerProductsConsumer", // Consumer tag
		true,                         // Auto-acknowledge
		false,                        // Exclusive
		false,                        // No-local
		false,                        // No-wait
		nil,                          // Args
	)
	if err != nil {
		log.Printf("Queue '%s' not available. Retrying in 5 seconds... Error: %v", queueName, err)
	}

	for msg := range msgs {
		messageBody := func() map[string]string { m := make(map[string]string); json.Unmarshal([]byte(msg.Body), &m); return m }()
		if messageBody["message"] == expectedMessage {
			fmt.Printf("akses di consumer scraping rabbitmq : %d ns\n", time.Now().UnixNano())
			scrapingControllerConsumer.Controller.ScrapeSoldSellerProducts(messageBody["seller"])
		} else {
			log.Printf("Message '%s' does not match expected message '%s'. Ignoring...", messageBody, expectedMessage)
		}
	}

	log.Println("Message channel closed, attempting to reconnect...")
}
