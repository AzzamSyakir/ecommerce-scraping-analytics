package routes

import (
	controller "etsy-trend-analytics/internal/controller/main"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

func Register(router *gin.Engine, c *controller.MainController) {
	categories := router.Group("/categories")
	{
		categories.GET("/trend", c.ProductCategoryTrends)
	}

}
func RunServer() {

	router := gin.Default()
	Register(router, &controller.MainController{})
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
