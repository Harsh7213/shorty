package routes

import (
	"net/http"

	"github.com/Harsh7213/shorty/auth"
	"github.com/Harsh7213/shorty/controllers"
)

func RegisterRoutes() {
	basePath := "/api/v1"
	http.HandleFunc("GET "+basePath+"/health", controllers.HealthCheck)
	http.HandleFunc("GET /{code}", controllers.GetLink)
	http.Handle("GET "+basePath+"/link/stats/{code}", auth.RequireJWT(http.HandlerFunc(controllers.GetLinkStats)))
	http.Handle("GET "+basePath+"/qr/{code}", auth.RequireJWT(http.HandlerFunc(controllers.GenerateQRCode)))
	http.HandleFunc("POST "+basePath+"/auth/register", controllers.RegisterUser)
	http.HandleFunc("POST "+basePath+"/auth/login", controllers.LoginUser)
	http.HandleFunc("POST "+basePath+"/auth/resend-verification", controllers.ResendVerificationEmail)
	http.HandleFunc("GET "+basePath+"/auth/verify", controllers.VerifyEmail)
	http.Handle("GET "+basePath+"/auth/me", auth.RequireJWT(http.HandlerFunc(controllers.CurrentUser)))
	http.Handle("PATCH "+basePath+"/auth/password", auth.RequireJWT(http.HandlerFunc(controllers.ChangePassword)))
	http.Handle("DELETE "+basePath+"/auth/account", auth.RequireJWT(http.HandlerFunc(controllers.DeleteAccount)))
	http.Handle("GET "+basePath+"/org", auth.RequireJWT(http.HandlerFunc(controllers.ListOrganizations)))
	http.Handle("POST "+basePath+"/org", auth.RequireJWT(http.HandlerFunc(controllers.CreateOrganization)))
	http.Handle("GET "+basePath+"/org/{organizationID}/members", auth.RequireJWT(http.HandlerFunc(controllers.ListOrganizationMembers)))
	http.Handle("POST "+basePath+"/org/{organizationID}/members", auth.RequireJWT(http.HandlerFunc(controllers.AddOrganizationMember)))
	http.Handle("DELETE "+basePath+"/org/{organizationID}/members/{userID}", auth.RequireJWT(http.HandlerFunc(controllers.RemoveOrganizationMember)))
	http.Handle("GET "+basePath+"/org/{organizationID}/links", auth.RequireJWT(http.HandlerFunc(controllers.ListOrganizationLinks)))
	http.Handle("POST "+basePath+"/link", auth.RequireJWT(http.HandlerFunc(controllers.CreateLink)))
	http.Handle("PATCH "+basePath+"/link/{code}", auth.RequireJWT(http.HandlerFunc(controllers.UpdateLink)))
	http.Handle("DELETE "+basePath+"/link/{code}", auth.RequireJWT(http.HandlerFunc(controllers.DeleteLink)))
}
