package controllers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Harsh7213/shorty/auth"
	"github.com/Harsh7213/shorty/db"
	"github.com/Harsh7213/shorty/handlers"
	"github.com/Harsh7213/shorty/models"
)

func CreateLink(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r)
	if !ok {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	var request struct {
		OrganizationID int64      `json:"organization_id"`
		LongURL        string     `json:"long_url"`
		ExpiresAt      *time.Time `json:"expires_at"`
	}

	if err := decodeJSON(w, r, &request); err != nil {
		return
	}

	if request.OrganizationID < 1 {
		http.Error(w, "A valid organization_id is required", http.StatusBadRequest)
		return
	}
	if request.LongURL == "" {
		http.Error(w, "Long URL is required", http.StatusBadRequest)
		return
	}
	if !handlers.IsValidURL(request.LongURL) {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}
	if request.ExpiresAt != nil && !request.ExpiresAt.After(time.Now()) {
		http.Error(w, "expires_at must be in the future", http.StatusBadRequest)
		return
	}

	tx, err := db.DB.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "Failed to create short link", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var lockedOrganizationID int64
	err = tx.QueryRowContext(r.Context(), `
		SELECT id
		FROM shorty.organizations
		WHERE id = $1
		FOR UPDATE`, request.OrganizationID).Scan(&lockedOrganizationID)
	if err != nil && err != sql.ErrNoRows {
		http.Error(w, "Failed to verify organization", http.StatusInternalServerError)
		return
	}
	if err == sql.ErrNoRows {
		http.Error(w, "Missing link.manage permission for this organization", http.StatusForbidden)
		return
	}

	var allowed bool
	err = tx.QueryRowContext(r.Context(), `
		SELECT EXISTS (
			SELECT 1
			FROM shorty.organizations
			WHERE id = $1 AND owner_user_id = $2
		) OR EXISTS (
			SELECT 1
			FROM shorty.organization_members
			WHERE organization_id = $1
			  AND user_id = $2
			  AND 'link.manage' = ANY(permissions)
		)`, request.OrganizationID, userID).Scan(&allowed)
	if err != nil {
		http.Error(w, "Failed to verify organization permission", http.StatusInternalServerError)
		return
	}
	if !allowed {
		http.Error(w, "Missing link.manage permission for this organization", http.StatusForbidden)
		return
	}

	var link models.Link
	var reused bool
	err = tx.QueryRowContext(r.Context(), `
		SELECT short_url_code
		FROM shorty.links
		WHERE organization_id = $1
		  AND long_url = $2
		  AND status = 'active'
		  AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY created_at DESC
		LIMIT 1`,
		request.OrganizationID, request.LongURL,
	).Scan(&link.ShortURLCode)
	if err == nil {
		reused = true
	} else if err != sql.ErrNoRows {
		http.Error(w, "Failed to check for an existing short link", http.StatusInternalServerError)
		return
	}

	if reused {
		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to create short link", http.StatusInternalServerError)
			return
		}
		link.ShortURL = os.Getenv("DOMAIN") + link.ShortURLCode
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(models.CreateLinkResponse{ShortURL: link.ShortURL}); err != nil {
			log.Printf("failed to write existing short link response: %v", err)
		}
		return
	}

	const maxRetries = 5
	var inserted bool
	for range maxRetries {
		code, err := handlers.GenerateShortURLCode(request.LongURL)
		if err != nil {
			http.Error(w, "Failed to generate short URL code", http.StatusInternalServerError)
			return
		}
		err = tx.QueryRowContext(r.Context(), `
			INSERT INTO shorty.links (
				organization_id, created_by_user_id, created_by_email, created_by_display_name,
				long_url, short_url_code, expires_at
			)
			SELECT $1, $2, u.email, u.display_name, $3, $4, $5
			FROM shorty.users u
			WHERE u.id = $2
			  AND (
				EXISTS (
					SELECT 1 FROM shorty.organizations
					WHERE id = $1 AND owner_user_id = $2
				)
				OR EXISTS (
					SELECT 1
					FROM shorty.organization_members
					WHERE organization_id = $1
					  AND user_id = $2
					  AND 'link.manage' = ANY(permissions)
				)
			  )
			ON CONFLICT (short_url_code) DO NOTHING
			RETURNING id, short_url_code, created_at`,
			request.OrganizationID, userID, request.LongURL, code, request.ExpiresAt,
		).Scan(&link.ID, &link.ShortURLCode, &link.CreatedAt)
		if err == nil {
			inserted = true
			break
		}
		if err != sql.ErrNoRows {
			http.Error(w, "Failed to create short link", http.StatusInternalServerError)
			return
		}
	}
	if !inserted {
		http.Error(w, "Unable to generate a unique short code", http.StatusInternalServerError)
		return
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, "Failed to create short link", http.StatusInternalServerError)
		return
	}

	link.ShortURL = os.Getenv("DOMAIN") + link.ShortURLCode
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	response := models.CreateLinkResponse{ShortURL: link.ShortURL}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("failed to write created short link response: %v", err)
	}
}

func GetLink(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	if code == "" {
		http.Error(w, "Short code is required", http.StatusBadRequest)
		return
	}

	var longURL string
	var linkID int64

	err := db.DB.QueryRow(`
		SELECT id, long_url
		FROM shorty.links
		WHERE short_url_code = $1
		AND status = 'active'
		AND (expires_at IS NULL OR expires_at > NOW())`, code).Scan(&linkID, &longURL)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Short URL not found", http.StatusNotFound)
			return
		}

		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Record click event
	if _, err = db.DB.Exec(`INSERT INTO shorty.click_events (link_id) VALUES ($1)`, linkID); err != nil {
		http.Error(w, "Failed to record click event", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, longURL, http.StatusFound)
}

func GetLinkStats(w http.ResponseWriter, r *http.Request) {
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

	var link models.Link
	var totalClicks int64

	err = db.DB.QueryRow(`
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

func ListOrganizationLinks(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r)
	if !ok {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	organizationID, err := strconv.ParseInt(r.PathValue("organizationID"), 10, 64)
	if err != nil || organizationID < 1 {
		http.Error(w, "A valid organizationID is required", http.StatusBadRequest)
		return
	}
	if !hasLinkPermission(w, userID, organizationID) {
		return
	}

	rows, err := db.DB.QueryContext(r.Context(), `
		SELECT l.id, l.long_url, l.short_url_code, l.status, l.expires_at,
		l.created_at, l.created_by_user_id, l.created_by_email,
		l.created_by_display_name, COUNT(c.id)
		FROM shorty.links l
		LEFT JOIN shorty.click_events c ON c.link_id = l.id
		WHERE l.organization_id = $1
		GROUP BY l.id
		ORDER BY l.created_at DESC`, organizationID)
	if err != nil {
		http.Error(w, "Failed to list organization links", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type organizationLink struct {
		ID           int64      `json:"id"`
		LongURL      string     `json:"long_url"`
		ShortURLCode string     `json:"short_url_code"`
		ShortURL     string     `json:"short_url"`
		Status       string     `json:"status"`
		ExpiresAt    *time.Time `json:"expires_at"`
		CreatedAt    time.Time  `json:"created_at"`
		CreatedBy    struct {
			UserID      *int64 `json:"user_id"`
			Email       string `json:"email"`
			DisplayName string `json:"display_name"`
		} `json:"created_by"`
		TotalClicks int64 `json:"total_clicks"`
	}
	links := make([]organizationLink, 0)
	for rows.Next() {
		var link organizationLink
		var expiresAt sql.NullTime
		var createdByUserID sql.NullInt64
		if err := rows.Scan(
			&link.ID, &link.LongURL, &link.ShortURLCode, &link.Status,
			&expiresAt, &link.CreatedAt, &createdByUserID,
			&link.CreatedBy.Email, &link.CreatedBy.DisplayName, &link.TotalClicks,
		); err != nil {
			http.Error(w, "Failed to read organization links", http.StatusInternalServerError)
			return
		}
		if createdByUserID.Valid {
			link.CreatedBy.UserID = &createdByUserID.Int64
		}
		if expiresAt.Valid {
			link.ExpiresAt = &expiresAt.Time
		}
		link.ShortURL = os.Getenv("DOMAIN") + link.ShortURLCode
		links = append(links, link)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "Failed to read organization links", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(links); err != nil {
		log.Printf("failed to write organization links response: %v", err)
	}
}

func DeleteLink(w http.ResponseWriter, r *http.Request) {
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

	result, err := db.DB.Exec(`
		UPDATE shorty.links
		SET status = 'revoked',
		    revoked_at = COALESCE(revoked_at, NOW()),
		    updated_at = NOW()
		WHERE short_url_code = $1
		AND organization_id = $2
		AND (
		    EXISTS (
		        SELECT 1 FROM shorty.organizations
		        WHERE id = $2 AND owner_user_id = $3
		    )
		    OR EXISTS (
		        SELECT 1
		        FROM shorty.organization_members
		        WHERE organization_id = $2
		          AND user_id = $3
		          AND 'link.manage' = ANY(permissions)
		    )
		)`, code, organizationID, userID)
	if err != nil {
		http.Error(w, "Failed to revoke link", http.StatusInternalServerError)
		return
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Error(w, "Failed to verify revocation", http.StatusInternalServerError)
		return
	}
	if rowsAffected == 0 {
		if !hasLinkPermission(w, userID, organizationID) {
			return
		}
		http.Error(w, "Short URL not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Short URL revoked successfully",
	})
}

func UpdateLink(w http.ResponseWriter, r *http.Request) {
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

	var request struct {
		Status    *string         `json:"status"`
		ExpiresAt json.RawMessage `json:"expires_at"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		return
	}
	if request.Status == nil && len(request.ExpiresAt) == 0 {
		http.Error(w, "At least one of status or expires_at is required", http.StatusBadRequest)
		return
	}

	if request.Status != nil {
		*request.Status = strings.ToLower(*request.Status)
		if *request.Status != "active" && *request.Status != "disabled" && *request.Status != "revoked" {
			http.Error(w, "status must be active, disabled, or revoked", http.StatusBadRequest)
			return
		}
	}

	var expiresAt *time.Time
	expiryProvided := len(request.ExpiresAt) > 0
	if expiryProvided && string(request.ExpiresAt) != "null" {
		var parsed time.Time
		if err := json.Unmarshal(request.ExpiresAt, &parsed); err != nil {
			http.Error(w, "expires_at must be a timestamp or null", http.StatusBadRequest)
			return
		}
		if !parsed.After(time.Now()) {
			http.Error(w, "expires_at must be in the future", http.StatusBadRequest)
			return
		}
		expiresAt = &parsed
	}

	organizationID, err := linkOrganizationID(code)
	if err != nil {
		writeLinkLookupError(w, err)
		return
	}
	if !hasLinkPermission(w, userID, organizationID) {
		return
	}

	var status any
	if request.Status != nil {
		status = *request.Status
	}
	var updatedStatus string
	var updatedExpiry *time.Time
	err = db.DB.QueryRow(`
		UPDATE shorty.links
		SET status = COALESCE($1, status),
		    revoked_at = CASE
		        WHEN $1 = 'revoked' THEN COALESCE(revoked_at, NOW())
		        WHEN $1 IN ('active', 'disabled') THEN NULL
		        ELSE revoked_at
		    END,
		    expires_at = CASE WHEN $2 THEN $3 ELSE expires_at END,
		    updated_at = NOW()
		WHERE short_url_code = $4
		AND organization_id = $5
		AND status <> 'revoked'
		AND (
		    EXISTS (
		        SELECT 1 FROM shorty.organizations
		        WHERE id = $5 AND owner_user_id = $6
		    )
		    OR EXISTS (
		        SELECT 1
		        FROM shorty.organization_members
		        WHERE organization_id = $5
		          AND user_id = $6
		          AND 'link.manage' = ANY(permissions)
		    )
		)
		RETURNING status, expires_at`,
		status, expiryProvided, expiresAt, code, organizationID, userID,
	).Scan(&updatedStatus, &updatedExpiry)
	if err != nil {
		if err == sql.ErrNoRows {
			if !hasLinkPermission(w, userID, organizationID) {
				return
			}
			http.Error(w, "Revoked links cannot be changed", http.StatusConflict)
			return
		}
		http.Error(w, "Failed to update link", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":     updatedStatus,
		"expires_at": updatedExpiry,
	})
}

func hasLinkPermission(w http.ResponseWriter, userID, organizationID int64) bool {
	var allowed bool
	err := db.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM shorty.organizations
			WHERE id = $1 AND owner_user_id = $2
		) OR EXISTS (
			SELECT 1
			FROM shorty.organization_members
			WHERE organization_id = $1
			  AND user_id = $2
			  AND 'link.manage' = ANY(permissions)
		)`, organizationID, userID).Scan(&allowed)
	if err != nil {
		http.Error(w, "Failed to verify organization permission", http.StatusInternalServerError)
		return false
	}
	if !allowed {
		http.Error(w, "Missing link.manage permission for this organization", http.StatusForbidden)
		return false
	}
	return true
}

func linkOrganizationID(code string) (int64, error) {
	var organizationID int64
	err := db.DB.QueryRow(
		`SELECT organization_id FROM shorty.links WHERE short_url_code = $1`,
		code,
	).Scan(&organizationID)
	return organizationID, err
}

func writeLinkLookupError(w http.ResponseWriter, err error) {
	if err == sql.ErrNoRows {
		http.Error(w, "Short URL not found", http.StatusNotFound)
		return
	}
	http.Error(w, "Database error", http.StatusInternalServerError)
}
