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

// CreateLink creates a short link or returns an existing link with the same destination and expiry.
// @Summary Create a short link
// @Tags Links
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreateLinkRequest true "Link destination and organization"
// @Success 201 {object} models.CreateLinkResponse
// @Success 200 {object} models.CreateLinkResponse "An existing matching link was reused"
// @Failure 400 {string} string "Invalid request"
// @Failure 401 {string} string "Authentication required"
// @Failure 403 {string} string "Missing link.manage permission"
// @Failure 409 {string} string "Link with a different expiry already exists"
// @Failure 500 {string} string "Failed to create short link"
// @Router /api/v1/link [post]
func CreateLink(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r)
	if !ok {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	var request CreateLinkRequest

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
	if request.ExpiresAt != nil {
		expiry := request.ExpiresAt.UTC()
		request.ExpiresAt = &expiry
	}
	if request.ExpiresAt != nil && !request.ExpiresAt.After(time.Now().UTC()) {
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
	var existingExpiryMatches bool
	err = tx.QueryRowContext(r.Context(), `
		SELECT short_url_code, expires_at IS NOT DISTINCT FROM $3
		FROM shorty.links
		WHERE organization_id = $1
		  AND long_url = $2
		  AND status = 'active'
		  AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY (expires_at IS NOT DISTINCT FROM $3) DESC, created_at DESC
		LIMIT 1`,
		request.OrganizationID, request.LongURL, request.ExpiresAt,
	).Scan(&link.ShortURLCode, &existingExpiryMatches)
	if err == nil {
		if !existingExpiryMatches {
			http.Error(w, "A short link for this destination already exists with a different expiry", http.StatusConflict)
			return
		}
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
		if err := json.NewEncoder(w).Encode(models.CreateLinkResponse{ShortURL: link.ShortURL, Reused: true}); err != nil {
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
	response := models.CreateLinkResponse{ShortURL: link.ShortURL, Reused: false}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("failed to write created short link response: %v", err)
	}
}

// GetLink redirects to the destination for an active, unexpired short link.
// @Summary Redirect a short link
// @Tags Links
// @Param code path string true "Short link code"
// @Success 302 "Redirect to the destination URL"
// @Failure 400 {string} string "Short code is required"
// @Failure 404 {string} string "Short URL not found"
// @Failure 500 {string} string "Failed to record click event"
// @Router /{code} [get]
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

	countryCode := requestCountryCode(r)
	if _, err = db.DB.Exec(`INSERT INTO shorty.click_events (link_id, country_code) VALUES ($1, $2)`, linkID, countryCode); err != nil {
		http.Error(w, "Failed to record click event", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, longURL, http.StatusFound)
}

// GetLinkStats returns click statistics for a short link.
// @Summary Get short link statistics
// @Description Available to all members of the link's organization; link.manage is only required to change links.
// @Tags Links
// @Produce json
// @Security BearerAuth
// @Param code path string true "Short link code"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {string} string "Authentication required"
// @Failure 403 {string} string "No access to this organization"
// @Failure 404 {string} string "Short URL not found"
// @Failure 500 {string} string "Database error"
// @Router /api/v1/link/stats/{code} [get]
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
	if !hasOrganizationReadAccess(w, userID, organizationID) {
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

// ListOrganizationLinks returns links and click summaries for an organization.
// @Summary List organization links
// @Description Available to all organization members. The link.manage permission is required only to create or change links.
// @Tags Links
// @Produce json
// @Security BearerAuth
// @Param organizationID path int true "Organization ID"
// @Success 200 {array} map[string]interface{}
// @Failure 400 {string} string "Invalid organization ID"
// @Failure 401 {string} string "Authentication required"
// @Failure 403 {string} string "No access to this organization"
// @Failure 500 {string} string "Failed to list organization links"
// @Router /api/v1/org/{organizationID}/links [get]
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
	if !hasOrganizationReadAccess(w, userID, organizationID) {
		return
	}

	rows, err := db.DB.QueryContext(r.Context(), `
		SELECT l.id, l.long_url, l.short_url_code, l.status, l.expires_at,
		l.created_at, l.created_by_user_id, l.created_by_email,
		l.created_by_display_name, COUNT(c.id),
		string_agg(DISTINCT c.country_code, ',' ORDER BY c.country_code)
			FILTER (WHERE c.country_code IS NOT NULL)
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
		TotalClicks  int64    `json:"total_clicks"`
		CountryCodes []string `json:"country_codes"`
	}
	links := make([]organizationLink, 0)
	for rows.Next() {
		var link organizationLink
		var expiresAt sql.NullTime
		var createdByUserID sql.NullInt64
		var countryCodes sql.NullString
		if err := rows.Scan(
			&link.ID, &link.LongURL, &link.ShortURLCode, &link.Status,
			&expiresAt, &link.CreatedAt, &createdByUserID,
			&link.CreatedBy.Email, &link.CreatedBy.DisplayName, &link.TotalClicks, &countryCodes,
		); err != nil {
			http.Error(w, "Failed to read organization links", http.StatusInternalServerError)
			return
		}
		link.CountryCodes = make([]string, 0)
		if countryCodes.Valid {
			link.CountryCodes = strings.Split(countryCodes.String, ",")
		}
		if createdByUserID.Valid {
			link.CreatedBy.UserID = &createdByUserID.Int64
		}
		if expiresAt.Valid {
			expiry := expiresAt.Time.UTC()
			link.ExpiresAt = &expiry
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

// GetLinkAnalytics returns daily and country click totals for an organization.
// @Summary Get link analytics
// @Description Available to all organization members, including members without link.manage.
// @Description By default, returns country totals without daily link series.
// @Description When link_id is specified, returns daily analytics only for that link.
// @Tags Links
// @Produce json
// @Security BearerAuth
// @Param organizationID path int true "Organization ID"
// @Param period query string false "Reporting window" Enums(week,month) default(week)
// @Param link_id query int false "Include daily analytics for this link only"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {string} string "Invalid organization ID, period, or link_id"
// @Failure 401 {string} string "Authentication required"
// @Failure 403 {string} string "No access to this organization"
// @Failure 500 {string} string "Failed to load analytics"
// @Router /api/v1/org/{organizationID}/analytics [get]
func GetLinkAnalytics(w http.ResponseWriter, r *http.Request) {
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
	if !hasOrganizationReadAccess(w, userID, organizationID) {
		return
	}

	period := r.URL.Query().Get("period")
	days := 7
	switch period {
	case "", "week":
		period = "week"
	case "month":
		days = 30
	default:
		http.Error(w, "period must be week or month", http.StatusBadRequest)
		return
	}

	var linkID int64
	if rawLinkID := r.URL.Query().Get("link_id"); rawLinkID != "" {
		linkID, err = strconv.ParseInt(rawLinkID, 10, 64)
		if err != nil || linkID < 1 {
			http.Error(w, "A valid link_id is required", http.StatusBadRequest)
			return
		}
	}

	type dailyClicks struct {
		Date   string `json:"date"`
		Clicks int64  `json:"clicks"`
	}
	type linkAnalytics struct {
		LinkID int64         `json:"link_id"`
		Clicks []dailyClicks `json:"clicks"`
	}
	type countryClicks struct {
		CountryCode string `json:"country_code"`
		Clicks      int64  `json:"clicks"`
	}
	links := make([]linkAnalytics, 0)
	if linkID != 0 {
		rows, err := db.DB.QueryContext(r.Context(), `
			SELECT l.id, TO_CHAR(calendar.day, 'YYYY-MM-DD'), COUNT(c.id)
			FROM shorty.links l
			CROSS JOIN GENERATE_SERIES(
				CURRENT_DATE - ($2::int - 1),
				CURRENT_DATE,
				INTERVAL '1 day'
			) AS calendar(day)
			LEFT JOIN shorty.click_events c
				ON c.link_id = l.id
				AND c.clicked_at >= calendar.day
				AND c.clicked_at < calendar.day + INTERVAL '1 day'
			WHERE l.organization_id = $1
			  AND l.id = $3
			GROUP BY l.id, l.created_at, calendar.day
			ORDER BY l.created_at DESC, l.id, calendar.day`,
			organizationID, days, linkID)
		if err != nil {
			http.Error(w, "Failed to load link analytics", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var resultLinkID int64
			var day dailyClicks
			if err := rows.Scan(&resultLinkID, &day.Date, &day.Clicks); err != nil {
				http.Error(w, "Failed to read link analytics", http.StatusInternalServerError)
				return
			}
			if len(links) == 0 || links[len(links)-1].LinkID != resultLinkID {
				links = append(links, linkAnalytics{
					LinkID: resultLinkID,
					Clicks: make([]dailyClicks, 0, days),
				})
			}
			links[len(links)-1].Clicks = append(links[len(links)-1].Clicks, day)
		}
		if err := rows.Err(); err != nil {
			http.Error(w, "Failed to read link analytics", http.StatusInternalServerError)
			return
		}
	}

	countries := make([]countryClicks, 0)
	if linkID == 0 {
		countryRows, err := db.DB.QueryContext(r.Context(), `
			SELECT COALESCE(c.country_code, ''), COUNT(*)
			FROM shorty.click_events c
			JOIN shorty.links l ON l.id = c.link_id
			WHERE l.organization_id = $1
			GROUP BY c.country_code
			ORDER BY COUNT(*) DESC, c.country_code ASC NULLS LAST`, organizationID)
		if err != nil {
			http.Error(w, "Failed to load country analytics", http.StatusInternalServerError)
			return
		}
		defer countryRows.Close()
		for countryRows.Next() {
			var country countryClicks
			if err := countryRows.Scan(&country.CountryCode, &country.Clicks); err != nil {
				http.Error(w, "Failed to read country analytics", http.StatusInternalServerError)
				return
			}
			countries = append(countries, country)
		}
		if err := countryRows.Err(); err != nil {
			http.Error(w, "Failed to read country analytics", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"period":    period,
		"links":     links,
		"countries": countries,
	}); err != nil {
		log.Printf("failed to write organization analytics response: %v", err)
	}
}

// DeleteLink revokes a short link.
// @Summary Revoke a short link
// @Tags Links
// @Produce json
// @Security BearerAuth
// @Param code path string true "Short link code"
// @Success 200 {object} map[string]string
// @Failure 401 {string} string "Authentication required"
// @Failure 403 {string} string "Missing link.manage permission"
// @Failure 404 {string} string "Short URL not found"
// @Failure 500 {string} string "Failed to revoke link"
// @Router /api/v1/link/{code} [delete]
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

// UpdateLink changes a short link's status and/or expiration.
// @Summary Update a short link
// @Tags Links
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param code path string true "Short link code"
// @Param body body UpdateLinkRequest true "Fields to update; use null to clear expires_at"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {string} string "Invalid request"
// @Failure 401 {string} string "Authentication required"
// @Failure 403 {string} string "Missing link.manage permission"
// @Failure 404 {string} string "Short URL not found"
// @Failure 409 {string} string "Expired links cannot be disabled or revoked links cannot be changed"
// @Failure 500 {string} string "Failed to update link"
// @Router /api/v1/link/{code} [patch]
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

	var request UpdateLinkRequest
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
		parsed = parsed.UTC()
		if !parsed.After(time.Now().UTC()) {
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
		AND ($1 IS DISTINCT FROM 'disabled' OR expires_at IS NULL OR expires_at > NOW())
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
			if request.Status != nil && *request.Status == "disabled" {
				var expired bool
				checkErr := db.DB.QueryRow(`
					SELECT status <> 'revoked' AND expires_at IS NOT NULL AND expires_at <= NOW()
					FROM shorty.links
					WHERE short_url_code = $1 AND organization_id = $2`, code, organizationID).Scan(&expired)
				if checkErr != nil && checkErr != sql.ErrNoRows {
					http.Error(w, "Failed to verify link expiration", http.StatusInternalServerError)
					return
				}
				if checkErr == nil && expired {
					http.Error(w, "Expired links cannot be disabled", http.StatusConflict)
					return
				}
			}
			http.Error(w, "Revoked links cannot be changed", http.StatusConflict)
			return
		}
		http.Error(w, "Failed to update link", http.StatusInternalServerError)
		return
	}
	if updatedExpiry != nil {
		expiry := updatedExpiry.UTC()
		updatedExpiry = &expiry
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":     updatedStatus,
		"expires_at": updatedExpiry,
	})
}

func requestCountryCode(r *http.Request) any {
	countryCode := strings.ToUpper(strings.TrimSpace(r.Header.Get("CF-IPCountry")))
	if len(countryCode) != 2 || countryCode == "XX" || countryCode == "T1" {
		return nil
	}
	for _, char := range countryCode {
		if char < 'A' || char > 'Z' {
			return nil
		}
	}
	return countryCode
}

func hasOrganizationReadAccess(w http.ResponseWriter, userID, organizationID int64) bool {
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
		)`, organizationID, userID).Scan(&allowed)
	if err != nil {
		http.Error(w, "Failed to verify organization access", http.StatusInternalServerError)
		return false
	}
	if !allowed {
		http.Error(w, "No access to this organization", http.StatusForbidden)
		return false
	}
	return true
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
