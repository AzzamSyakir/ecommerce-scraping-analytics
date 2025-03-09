package container

import (
	"ecommerce-scraping-analytics/internal/config"
	"ecommerce-scraping-analytics/internal/controllers"
	"ecommerce-scraping-analytics/internal/middleware"
	"ecommerce-scraping-analytics/internal/rabbitmq/consumer"
	"ecommerce-scraping-analytics/internal/rabbitmq/producer"
	"ecommerce-scraping-analytics/internal/routes"
	"log"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

type Container struct {
	Env        *config.EnvConfig
	Db         *config.DatabaseConfig
	Controller *ControllerContainer
	RabbitMq   *config.RabbitMqConfig
	Route      *routes.Route
	Middleware *middleware.Middleware
}

func NewContainer() *Container {
	err := godotenv.Load()

	if err != nil {
		log.Fatal("Error loading .env file: ", err)
	}
	envConfig := config.NewEnvConfig()
	dbConfig := config.NewDBConfig(envConfig)
	rabbitmqConfig := config.NewRabbitMqConfig(envConfig)
	logicController := controllers.NewLogicController(nil)
	mainControllerProducer := producer.CreateNewMainControllerProducer(envConfig)
	scrapingControllerProducer := producer.CreateNewScrapingControllerProducer()
	mainController := controllers.NewMainController(logicController, rabbitmqConfig, mainControllerProducer)
	scrapingController := controllers.NewScrapingController(rabbitmqConfig, scrapingControllerProducer)
	controllerContainer := NewControllerContainer(logicController, mainController, scrapingController)
	consumer := consumer.NewConsumerEntrypointInit(rabbitmqConfig, mainController, scrapingController, envConfig)
	consumer.ConsumerEntrypointStart()
	router := mux.NewRouter()
	middleware := middleware.NewMiddleware()
	routeConfig := routes.NewRoute(
		router,
		logicController,
		scrapingController,
		mainController,
	)
	routeConfig.Register()
	container := &Container{
		Db:         dbConfig,
		Controller: controllerContainer,
		RabbitMq:   rabbitmqConfig,
		Route:      routeConfig,
		Env:        envConfig,
		Middleware: middleware,
	}
	return container
}
