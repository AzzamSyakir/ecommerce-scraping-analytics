package controller

import (
	"etsy-trend-analytics/internal/config"
	"etsy-trend-analytics/internal/controller/logic"
	"etsy-trend-analytics/internal/controller/scraping"

	"github.com/gin-gonic/gin"
)

type MainController struct {
	logicController *logic.LogicController
	rabbitmq        *config.RabbitMqConfig
}

func NewMainController(logic *logic.LogicController, scraping scraping.ScrapingController) *MainController {
	return &MainController{logicController: logic}
}

func (mainController *MainController) ProductCategoryTrends(c *gin.Context) {
}
