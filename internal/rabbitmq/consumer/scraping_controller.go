package consumer

import (
	"etsy-trend-analytics/internal/config"
	"etsy-trend-analytics/internal/controllers"
	"log"
)

type ScrapingControllerConsumer struct {
	Controller *controllers.ScrapingController
}

func (scrapingControllerConsumer *ScrapingControllerConsumer) ConsumeMessageProductCategory(rabbitMQConfig *config.RabbitMqConfig) {
	queueName := "ProductCategoyTrends Queue"
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
		q.Name,              // Queue name
		"Scraping Consumer", // Consumer tag
		true,                // Auto-acknowledge
		false,               // Exclusive
		false,               // No-local
		false,               // No-wait
		nil,                 // Args
	)
	if err != nil {
		log.Printf("Queue '%s' not available. Retrying in 5 seconds... Error: %v", queueName, err)
	}

	log.Printf("Consumer started for queue: %s", queueName)

	for msg := range msgs {
		messageBody := string(msg.Body)
		if messageBody == expectedMessage {
			log.Println("Expected message received. Starting scraping process...")
			scrapingControllerConsumer.Controller.ProductCategoryTrendsScrapingController()
		} else {
			log.Printf("Message '%s' does not match expected message '%s'. Ignoring...", messageBody, expectedMessage)
		}
	}

	log.Println("Message channel closed, attempting to reconnect...")
}
