package routes

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

func RunServer() {

	router := gin.Default()
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
