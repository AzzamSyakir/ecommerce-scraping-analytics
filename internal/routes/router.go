package routes

import (
	"ecommerce-scraping-analytics/internal/controllers"

	"github.com/gorilla/mux"
)

type Route struct {
	Router             *mux.Router
	LogicController    *controllers.LogicController
	ScrapingController *controllers.ScrapingController
	MainController     *controllers.MainController
}

func NewRoute(router *mux.Router, logic *controllers.LogicController, scraping *controllers.ScrapingController, main *controllers.MainController) *Route {
	route := &Route{
		Router:             router.PathPrefix("/api").Subrouter(),
		LogicController:    logic,
		ScrapingController: scraping,
		MainController:     main,
	}
	return route
}

func (route *Route) Register() {
	route.Router.HandleFunc("/get-all-products/{seller}", route.MainController.GetAllSellerProducts).Methods("GET")
	route.Router.HandleFunc("/get-sold-products/{seller}", route.MainController.GetSoldSellerProducts).Methods("GET")
}
