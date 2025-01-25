package main

import (
	"ecommerce-scraping-analytics/internal/container"
	"fmt"
	"log"
	"net/http"
)

func main() {

	fmt.Println("App Started")
	container := container.NewContainer()
	// http server
	address := fmt.Sprintf(
		"%s:%s",
		"0.0.0.0",
		container.Env.App.AppPort,
	)
	listenAndServeErr := http.ListenAndServe(address, container.Route.Router)
	if listenAndServeErr != nil {
		log.Fatalf("failed to serve HTTP: %v", listenAndServeErr)
	}
	fmt.Println("app finish")

}
