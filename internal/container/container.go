package container

import (
	"ecommerce-scraping-analytics/internal/config"
	"ecommerce-scraping-analytics/internal/controllers"
	"ecommerce-scraping-analytics/internal/rabbitmq/consumer"
	"ecommerce-scraping-analytics/internal/rabbitmq/producer"
	"ecommerce-scraping-analytics/internal/routes"
	"log"
	"os"

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
		log.Fatal("Error loading .env file: ", err)
	}
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}
	dbConfig := config.NewDBConfig()
	if dbConfig.DB != nil {
		rabbitmqConfig := config.NewRabbitMqConfig()
		logicController := controllers.NewLogicController(dbConfig.DB.Connection)
		mainControllerProducer := producer.CreateNewMainControllerProducer()
		scrapingControllerProducer := producer.CreateNewScrapingControllerProducer()
		mainController := controllers.NewMainController(logicController, rabbitmqConfig, mainControllerProducer)
		scrapingController := controllers.NewScrapingController(rabbitmqConfig, scrapingControllerProducer)
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
	rabbitmqConfig := config.NewRabbitMqConfig()
	logicController := controllers.NewLogicController(nil)
	mainControllerProducer := producer.CreateNewMainControllerProducer()
	scrapingControllerProducer := producer.CreateNewScrapingControllerProducer()
	mainController := controllers.NewMainController(logicController, rabbitmqConfig, mainControllerProducer)
	scrapingController := controllers.NewScrapingController(rabbitmqConfig, scrapingControllerProducer)
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
