package container

import "ecommerce-scraping-analytics/internal/controllers"

type ControllerContainer struct {
	LogicController    *controllers.LogicController
	MainController     *controllers.MainController
	ScrapingController *controllers.ScrapingController
}

func NewControllerContainer(logicController *controllers.LogicController, mainController *controllers.MainController, scrapingController *controllers.ScrapingController) *ControllerContainer {
	controllerContainer := &ControllerContainer{
		LogicController:    logicController,
		MainController:     mainController,
		ScrapingController: scrapingController,
	}
	return controllerContainer
}
