package routes

import (
	"net/http"

	"github.com/Harsh7213/shorty/controllers"
)

func RegisterRoutes() {
	basePath := "/api/v1"
	http.HandleFunc("GET "+basePath+"/health", controllers.HealthCheck)
	http.HandleFunc("GET /{code}", controllers.GetLink)
	http.HandleFunc("GET "+basePath+"/link/stats/{code}", controllers.GetLinkStats)
	http.HandleFunc("POST "+basePath+"/link", controllers.CreateLink)
	http.HandleFunc("DELETE "+basePath+"/link/{code}", controllers.DeleteLink)
}
