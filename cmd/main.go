package main

import (
	"etsy-trend-analytics/internal/container"
	"fmt"
)

func main() {
	fmt.Println("App Started")
	container.NewContainer()
	fmt.Println("app finish")
}
