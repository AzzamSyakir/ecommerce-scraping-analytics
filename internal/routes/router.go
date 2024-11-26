package routes

import (
	"ecommerce-scraping-analytics/internal/controllers"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

type Route struct {
	Router             *gin.Engine
	LogicController    *controllers.LogicController
	ScrapingController *controllers.ScrapingController
	MainController     *controllers.MainController
}

func NewRoute(router *gin.Engine, logic *controllers.LogicController, scraping *controllers.ScrapingController, main *controllers.MainController) *Route {
	route := &Route{
		Router:             router,
		LogicController:    logic,
		ScrapingController: scraping,
		MainController:     main,
	}
	return route
}

func Register(router *gin.Engine, c *controllers.MainController) {
	categories := router.Group("/api")
	{
		categories.GET("/trend-product/:seller", c.GetPopularProductsBySeller)
	}

}
func (route *Route) RunServer() {

	router := gin.Default()
	Register(router, route.MainController)
	server := &http.Server{
		Addr:           fmt.Sprintf("%s:%s", os.Getenv("GATEWAY_APP_HOST"), os.Getenv("GATEWAY_APP_PORT")),
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	err := server.ListenAndServe()
	if err != nil {
		panic(err)
	}

}
