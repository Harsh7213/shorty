package controllers

import (
	"database/sql"
	"net/http"
	"os"

	"github.com/Harsh7213/shorty/auth"
	"github.com/Harsh7213/shorty/db"
	qr "github.com/Harsh7213/shorty/qrcode"
)

func GenerateQRCode(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r)
	if !ok {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	code := r.PathValue("code")
	if code == "" {
		http.Error(w, "Short code is required", http.StatusBadRequest)
		return
	}
	organizationID, err := linkOrganizationID(code)
	if err != nil {
		writeLinkLookupError(w, err)
		return
	}
	if !hasLinkPermission(w, userID, organizationID) {
		return
	}

	var storedCode string
	err = db.DB.QueryRow(`
		SELECT short_url_code
		FROM shorty.links
		WHERE short_url_code = $1
		AND status = 'active'
		AND (expires_at IS NULL OR expires_at > NOW())`, code).Scan(&storedCode)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Short URL not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	shortURL := os.Getenv("DOMAIN") + storedCode
	image, err := qr.GenerateQR(shortURL)
	if err != nil {
		http.Error(w, "Failed to generate QR code", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(image)
}
