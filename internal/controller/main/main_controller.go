package controller

import (
	"etsy-trend-analytics/internal/controller/logic"
	"etsy-trend-analytics/internal/controller/scraping"

	"github.com/gin-gonic/gin"
	"github.com/streadway/amqp"
)

type MainController struct {
	logicController *logic.LogicController
	rabbitmq        *amqp.Connection
}

func NewMainController(logic *logic.LogicController, scraping scraping.ScrapingController) *MainController {
	return &MainController{logicController: logic}
}

func ProductCategoryTrends() (c *gin.Context) {

}
