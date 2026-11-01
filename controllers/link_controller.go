package controllers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/Harsh7213/shorty/db"
	"github.com/Harsh7213/shorty/handlers"
	"github.com/Harsh7213/shorty/models"
)

func CreateLink(w http.ResponseWriter, r *http.Request) {
	var link models.Link

	if err := json.NewDecoder(r.Body).Decode(&link); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if link.LongURL == "" {
		http.Error(w, "Long URL is required", http.StatusBadRequest)
		return
	}

	//Validation URL Format
	if !handlers.IsValidURL(link.LongURL) {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}

	// Check whether long URL already exists
	queryCheck := `SELECT id, long_url, short_url_code, created_at FROM shorty.links WHERE long_url = $1`

	err := db.DB.QueryRow(queryCheck, link.LongURL).Scan(
		&link.ID,
		&link.LongURL,
		&link.ShortURLCode,
		&link.CreatedAt,
	)

	if err != nil && err != sql.ErrNoRows {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	const maxRetries = 5
	inserted := false
	// If the long URL does not exist, create a new short url
	if err == sql.ErrNoRows {
		for range maxRetries {
			link.ShortURLCode, err = handlers.GenerateShortURLCode(link.LongURL)
			if err != nil {
				http.Error(w, "Failed to generate short URL code", http.StatusInternalServerError)
				return
			}
			query := `INSERT INTO shorty.links (long_url, short_url_code) VALUES ($1, $2) 
			ON CONFLICT DO NOTHING
			RETURNING id, long_url, short_url_code, created_at`

			err = db.DB.QueryRow(query, link.LongURL, link.ShortURLCode).Scan(
				&link.ID,
				&link.LongURL,
				&link.ShortURLCode,
				&link.CreatedAt,
			)

			if err == nil {
				inserted = true
				break
			}
			if err != sql.ErrNoRows {
				http.Error(w, "Failed to create short link", http.StatusInternalServerError)
				return
			}
			// Check if another request inserted the same long URL (race condition)
			queryCheck := ` SELECT id, long_url, short_url_code, created_at FROM shorty.links WHERE long_url = $1 `

			err = db.DB.QueryRow(queryCheck, link.LongURL).Scan(
				&link.ID,
				&link.LongURL,
				&link.ShortURLCode,
				&link.CreatedAt,
			)

			if err == nil {
				inserted = true
				break
			}

			if err != sql.ErrNoRows {
				http.Error(w, "Database error", http.StatusInternalServerError)
				return
			}
		}
		if !inserted {
			http.Error(w, "Unable to generate unique short One small HTTP improvement: Since your endpoint can return either an existing link or create a new one, use 200 OK for existing links and 201 Created for newly inserted links rather than always returning 201code", http.StatusInternalServerError)
			return
		}

	}
	link.ShortURL = os.Getenv("DOMAIN") + link.ShortURLCode
	w.Header().Set("Content-Type", "application/json")
	if inserted {
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	response := models.CreateLinkResponse{ShortURL: link.ShortURL}
	json.NewEncoder(w).Encode(response)
}

func GetLink(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	if code == "" {
		http.Error(w, "Short code is required", http.StatusBadRequest)
		return
	}

	var longURL string
	var linkID int64

	err := db.DB.QueryRow(`SELECT id, long_url FROM shorty.links WHERE short_url_code = $1 `, code).Scan(&linkID, &longURL)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Short URL not found", http.StatusNotFound)
			return
		}

		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Record click event
	_, err = db.DB.Exec(`INSERT INTO shorty.click_events (link_id) VALUES ($1)`, linkID)

	if err != nil {
		log.Fatal("Failed to record click event: ", err)
		return
	}

	http.Redirect(w, r, longURL, http.StatusFound)
}

func GetLinkStats(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	if code == "" {
		http.Error(w, "Short code is required", http.StatusBadRequest)
		return
	}

	var link models.Link
	var totalClicks int64

	err := db.DB.QueryRow(`
        SELECT l.id, l.long_url, l.short_url_code, l.created_at, COUNT(c.id) AS total_clicks
        FROM shorty.links l
        LEFT JOIN shorty.click_events c ON l.id = c.link_id
        WHERE l.short_url_code = $1
        GROUP BY l.id`, code).Scan(
		&link.ID,
		&link.LongURL,
		&link.ShortURLCode,
		&link.CreatedAt,
		&totalClicks,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Short URL not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	link.ShortURL = os.Getenv("DOMAIN") + link.ShortURLCode

	response := map[string]any{
		"link":         link,
		"total_clicks": totalClicks,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func DeleteLink(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	if code == "" {
		http.Error(w, "Short code is required", http.StatusBadRequest)
		return
	}

	result, err := db.DB.Exec(`DELETE FROM shorty.links WHERE short_url_code = $1`, code)

	if err != nil {
		http.Error(w, "Failed to delete link", http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Error(w, "Failed to verify deletion", http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		http.Error(w, "Short URL not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(map[string]string{
		"message": "Short URL deleted successfully",
	})
}
