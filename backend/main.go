package main

import (
	"log"
	"net/http"
	"os"

	"github.com/Harsh7213/shorty/db"
	_ "github.com/Harsh7213/shorty/docs"
	"github.com/Harsh7213/shorty/routes"
)

// @title Shorty
// @version v1.0
// @description URL shortener API.
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter "Bearer" followed by a space and your access token.
func main() {
	_, err := db.ConnectDB()
	if err != nil {
		log.Fatal(err)
	}
	defer db.CloseDB()
	routes.RegisterRoutes()
	log.Println("Server is running on port ", os.Getenv("PORT"))
	log.Fatal(http.ListenAndServe(":"+os.Getenv("PORT"), nil))
}
