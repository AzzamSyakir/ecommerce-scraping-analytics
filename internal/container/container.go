package container

import (
	"etsy-trend-analytics/internal/config"
	"etsy-trend-analytics/internal/controllers"
	"etsy-trend-analytics/internal/rabbitmq/consumer"
	"etsy-trend-analytics/internal/rabbitmq/producer"
	"etsy-trend-analytics/internal/routes"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

type Container struct {
	Db         *config.DatabaseConfig
	Controller *ControllerContainer
	RabbitMq   *config.RabbitMqConfig
	Route      *routes.Route
}

func NewContainer() *Container {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	dbConfig := config.NewDBConfig()
	rabbitmqConfig := config.NewRabbitMqConfig()
	logicController := controllers.NewLogicController(dbConfig.DB.Connection)
	mainControllerProducer := producer.CreateNewMainControllerProducer()
	mainController := controllers.NewMainController(logicController, rabbitmqConfig, mainControllerProducer)
	scrapingController := controllers.NewScrapingController()
	controllerContainer := NewControllerContainer(logicController, mainController, scrapingController)
	consumer := consumer.NewConsumerEntrypointInit(rabbitmqConfig, mainController, scrapingController)
	consumer.ConsumerEntrypointStart()
	router := gin.Default()
	routeConfig := routes.NewRoute(
		router,
		logicController,
		scrapingController,
		mainController,
	)
	routeConfig.RunServer()
	container := &Container{
		Db:         dbConfig,
		Controller: controllerContainer,
		RabbitMq:   rabbitmqConfig,
		Route:      routeConfig,
	}
	return container
}
