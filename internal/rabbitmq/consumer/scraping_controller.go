package consumer

import (
	"ecommerce-scraping-analytics/internal/config"
	"ecommerce-scraping-analytics/internal/controllers"
	"encoding/json"
	"log"
)

type ScrapingControllerConsumer struct {
	Controller *controllers.ScrapingController
}

func (scrapingControllerConsumer *ScrapingControllerConsumer) ConsumeMessageAllSellerProduct(rabbitMQConfig *config.RabbitMqConfig) {
	queueName := "GetAllSellerProduct Queue"
	q, err := rabbitMQConfig.Channel.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		log.Printf("Failed to declare a queue:%v", err)
	}
	expectedMessage := "Start Scraping"
	msgs, err := rabbitMQConfig.Channel.Consume(
		q.Name,                       // Queue name
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

	log.Printf("Consumer started for queue: %s", queueName)

	for msg := range msgs {
		messageBody := func() map[string]string { m := make(map[string]string); json.Unmarshal([]byte(msg.Body), &m); return m }()
		if messageBody["message"] == expectedMessage {
			log.Println("Expected message received. Starting scraping process...")
			scrapingControllerConsumer.Controller.ScrapeAllSellerProducts(messageBody["seller"])
		} else {
			log.Printf("Message '%s' does not match expected message '%s'. Ignoring...", messageBody, expectedMessage)
		}
	}

	log.Println("Message channel closed, attempting to reconnect...")
}
func (scrapingControllerConsumer *ScrapingControllerConsumer) ConsumeMessageSoldSellerProduct(rabbitMQConfig *config.RabbitMqConfig) {
	queueName := "GetSoldSellerProduct Queue"
	q, err := rabbitMQConfig.Channel.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		log.Printf("Failed to declare a queue:%v", err)
	}
	expectedMessage := "Start Scraping"
	msgs, err := rabbitMQConfig.Channel.Consume(
		q.Name,                        // Queue name
		"SoldSellerProducts Consumer", // Consumer tag
		true,                          // Auto-acknowledge
		false,                         // Exclusive
		false,                         // No-local
		false,                         // No-wait
		nil,                           // Args
	)
	if err != nil {
		log.Printf("Queue '%s' not available. Retrying in 5 seconds... Error: %v", queueName, err)
	}

	log.Printf("Consumer started for queue: %s", queueName)

	for msg := range msgs {
		messageBody := func() map[string]string { m := make(map[string]string); json.Unmarshal([]byte(msg.Body), &m); return m }()
		if messageBody["message"] == expectedMessage {
			log.Println("Expected message received. Starting scraping process...")
			scrapingControllerConsumer.Controller.ScrapeSoldSellerProducts(messageBody["seller"])
		} else {
			log.Printf("Message '%s' does not match expected message '%s'. Ignoring...", messageBody, expectedMessage)
		}
	}

	log.Println("Message channel closed, attempting to reconnect...")
}
