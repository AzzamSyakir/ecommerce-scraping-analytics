package main

import (
	"ecommerce-scraping-analytics/internal/container"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
)

func main() {
	go func() {
		log.Println("Starting pprof server on localhost:6060")
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	fmt.Println("App Started")
	container.NewContainer()
	fmt.Println("app finish")
}
