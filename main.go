package main

import (
	"log"
	"net/http"
	"os"

	"github.com/Harsh7213/shorty/db"
	"github.com/Harsh7213/shorty/routes"
)

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
