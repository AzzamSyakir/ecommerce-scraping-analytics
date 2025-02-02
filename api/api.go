package main

import (
	_ "ecommerce-scraping-analytics/api/docs"
)

// @title Swagger Example API
// @version 1.0
// @description This is a sample server celler server.
// @tag.name This is the name of the tag
// @tag.description Cool Description
// @tag.docs.url https://example.com
// @tag.docs.description Best example documentation
// @termsOfService http://swagger.io/terms/
// @contact.name Azzam Syakir
// @contact.url https://github.com/AzzamSyakir
// @contact.email azzam.sykir.work@gmail.com
// @host localhost:8080
// func main() {
// 	r := mux.NewRouter()

// 	r.PathPrefix("/swagger/*").Handler(httpSwagger.Handler(
// 		httpSwagger.URL("http://localhost:1323/api/docs/doc.json"), //The url pointing to API definition
// 		httpSwagger.DeepLinking(true),
// 		httpSwagger.DocExpansion("none"),
// 		httpSwagger.DomID("swagger-ui"),
// 	)).Methods(http.MethodGet)
// 	fmt.Println("start")
// 	log.Fatal(http.ListenAndServe(":1323", r))
// 	fmt.Println("finish")
// }
