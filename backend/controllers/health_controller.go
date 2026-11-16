package controllers

import (
	"encoding/json"
	"net/http"
)

// HealthCheck reports whether the API is healthy.
// @Summary Check API health
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /api/v1/health [get]
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]string{"status": "healthy"}
	json.NewEncoder(w).Encode(response)
}
