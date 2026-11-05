package controllers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Harsh7213/shorty/auth"
	"github.com/Harsh7213/shorty/db"
	"github.com/Harsh7213/shorty/models"
	"github.com/Harsh7213/shorty/notifications"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

func CreateOrganization(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r)
	if !ok {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	var request struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		return
	}
	if strings.TrimSpace(request.Name) == "" {
		http.Error(w, "Organization name is required", http.StatusBadRequest)
		return
	}

	tx, err := db.DB.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "Failed to create organization", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var organization models.Organization
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO shorty.organizations (name, owner_user_id)
		VALUES ($1, $2)
		RETURNING id, name, owner_user_id, created_at`,
		strings.TrimSpace(request.Name), userID,
	).Scan(&organization.ID, &organization.Name, &organization.OwnerUserID, &organization.CreatedAt)
	if err != nil {
		http.Error(w, "Failed to create organization", http.StatusInternalServerError)
		return
	}

	_, err = tx.ExecContext(r.Context(), `
		INSERT INTO shorty.organization_members
			(organization_id, user_id, permissions)
		VALUES ($1, $2, ARRAY[]::TEXT[])`,
		organization.ID, userID,
	)
	if err != nil {
		if isUniqueViolation(err) {
			http.Error(w, "An account can belong to only one organization", http.StatusConflict)
			return
		}
		http.Error(w, "Failed to create organization membership", http.StatusInternalServerError)
		return
	}

	if err = tx.Commit(); err != nil {
		http.Error(w, "Failed to create organization", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(organization); err != nil {
		log.Printf("failed to write created organization response: %v", err)
	}
}

func ListOrganizations(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserID(r)
	if !ok {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	rows, err := db.DB.QueryContext(r.Context(), `
		SELECT o.id, o.name, o.owner_user_id, o.created_at
		FROM shorty.organizations o
		JOIN shorty.organization_members m
		  ON m.organization_id = o.id
		WHERE m.user_id = $1
		ORDER BY o.id`, userID)
	if err != nil {
		http.Error(w, "Failed to list organizations", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	organizations := make([]models.Organization, 0)
	for rows.Next() {
		var organization models.Organization
		if err := rows.Scan(&organization.ID, &organization.Name, &organization.OwnerUserID, &organization.CreatedAt); err != nil {
			http.Error(w, "Failed to read organizations", http.StatusInternalServerError)
			return
		}
		organizations = append(organizations, organization)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "Failed to read organizations", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(organizations); err != nil {
		log.Printf("failed to write organization list response: %v", err)
	}
}

func AddOrganizationMember(w http.ResponseWriter, r *http.Request) {
	requesterID, ok := auth.UserID(r)
	if !ok {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	organizationID, ok := organizationIDFromPath(w, r)
	if !ok {
		return
	}
	var request struct {
		Email       string   `json:"email"`
		DisplayName string   `json:"display_name"`
		Password    string   `json:"password"`
		Permissions []string `json:"permissions"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		return
	}
	email, valid := normalizeEmail(request.Email)
	if !valid {
		http.Error(w, "A valid email address is required", http.StatusBadRequest)
		return
	}
	if len(request.DisplayName) > 100 {
		http.Error(w, "display_name must be at most 100 characters", http.StatusBadRequest)
		return
	}
	for _, permission := range request.Permissions {
		if permission != "link.manage" && permission != "organization.manage" {
			http.Error(w, "Unsupported organization permission", http.StatusBadRequest)
			return
		}
	}
	if request.Permissions == nil {
		request.Permissions = []string{}
	}

	tx, err := db.DB.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "Failed to add organization member", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var lockedOrganizationID int64
	err = tx.QueryRowContext(r.Context(), `
		SELECT id FROM shorty.organizations WHERE id = $1 FOR UPDATE`,
		organizationID,
	).Scan(&lockedOrganizationID)
	if err != nil {
		http.Error(w, "Organization not found", http.StatusNotFound)
		return
	}
	allowed, err := organizationPermissionExists(r.Context(), tx, requesterID, organizationID, "organization.manage")
	if err != nil {
		http.Error(w, "Failed to verify organization permission", http.StatusInternalServerError)
		return
	}
	if !allowed {
		http.Error(w, "Missing organization.manage permission", http.StatusForbidden)
		return
	}
	requesterIsOwner, err := organizationOwnerExists(r.Context(), tx, requesterID, organizationID)
	if err != nil {
		http.Error(w, "Failed to verify organization owner", http.StatusInternalServerError)
		return
	}

	var memberID int64
	var emailVerified, deactivated, passwordSetupRequired bool
	err = tx.QueryRowContext(r.Context(), `
		SELECT id, email_verified_at IS NOT NULL, deactivated_at IS NOT NULL, password_setup_required
		FROM shorty.users WHERE email = $1 FOR UPDATE`,
		email,
	).Scan(&memberID, &emailVerified, &deactivated, &passwordSetupRequired)
	if errors.Is(err, sql.ErrNoRows) {
		if len(request.Password) < 12 || len(request.Password) > 72 {
			http.Error(w, "password must be between 12 and 72 bytes when creating a new user", http.StatusBadRequest)
			return
		}
		passwordHash, hashErr := bcrypt.GenerateFromPassword([]byte(request.Password), bcryptCost)
		if hashErr != nil {
			http.Error(w, "Failed to add organization member", http.StatusInternalServerError)
			return
		}
		err = tx.QueryRowContext(r.Context(), `
			INSERT INTO shorty.users (email, display_name, password_hash, password_setup_required)
			VALUES ($1, $2, $3, TRUE)
			ON CONFLICT (email) DO NOTHING
			RETURNING id, email_verified_at IS NOT NULL, deactivated_at IS NOT NULL, password_setup_required`,
			email, strings.TrimSpace(request.DisplayName), string(passwordHash),
		).Scan(&memberID, &emailVerified, &deactivated, &passwordSetupRequired)
		if errors.Is(err, sql.ErrNoRows) {
			err = tx.QueryRowContext(r.Context(), `
				SELECT id, email_verified_at IS NOT NULL, deactivated_at IS NOT NULL, password_setup_required
				FROM shorty.users WHERE email = $1 FOR UPDATE`,
				email,
			).Scan(&memberID, &emailVerified, &deactivated, &passwordSetupRequired)
		}
	}
	if err != nil {
		http.Error(w, "Failed to add organization member", http.StatusInternalServerError)
		return
	}
	memberIsOwner, err := organizationOwnerExists(r.Context(), tx, memberID, organizationID)
	if err != nil {
		http.Error(w, "Failed to verify organization owner", http.StatusInternalServerError)
		return
	}
	if memberIsOwner && len(request.Permissions) > 0 {
		http.Error(w, "The organization owner is managed separately from members and admins", http.StatusConflict)
		return
	}
	if deactivated {
		http.Error(w, "Account is deactivated", http.StatusConflict)
		return
	}
	if !emailVerified {
		verificationToken, tokenErr := createVerificationToken(r, tx, memberID)
		if tokenErr != nil {
			http.Error(w, "Failed to create verification token", http.StatusInternalServerError)
			return
		}

		var currentPermissions []string
		err = tx.QueryRowContext(r.Context(), `
			SELECT permissions FROM shorty.organization_members
			WHERE organization_id = $1 AND user_id = $2`,
			organizationID, memberID,
		).Scan(pq.Array(&currentPermissions))
		if err != nil && err != sql.ErrNoRows {
			http.Error(w, "Failed to read organization membership", http.StatusInternalServerError)
			return
		}
		membershipExists := err == nil
		if !requesterIsOwner && membershipExists && containsPermission(currentPermissions, "organization.manage") &&
			!containsPermission(request.Permissions, "organization.manage") {
			var activeOtherManagers int
			err = tx.QueryRowContext(r.Context(), `
				SELECT COUNT(*)
				FROM shorty.organization_members m
				JOIN shorty.users u ON u.id = m.user_id
				WHERE m.organization_id = $1 AND m.user_id <> $2
				  AND 'organization.manage' = ANY(m.permissions)
				  AND u.deactivated_at IS NULL
				  AND u.email_verified_at IS NOT NULL`,
				organizationID, memberID,
			).Scan(&activeOtherManagers)
			if err != nil {
				http.Error(w, "Failed to verify organization managers", http.StatusInternalServerError)
				return
			}
			if activeOtherManagers == 0 {
				http.Error(w, "Cannot remove the last active organization manager", http.StatusConflict)
				return
			}
		}
		if _, err = tx.ExecContext(r.Context(), `
			INSERT INTO shorty.organization_members (organization_id, user_id, permissions)
			VALUES ($1, $2, $3)
			ON CONFLICT (organization_id, user_id) DO UPDATE
			SET permissions = EXCLUDED.permissions`,
			organizationID, memberID, pq.Array(request.Permissions),
		); err != nil {
			if isUniqueViolation(err) {
				http.Error(w, "An account can belong to only one organization", http.StatusConflict)
				return
			}
			http.Error(w, "Failed to add organization member", http.StatusInternalServerError)
			return
		}
		if err = tx.Commit(); err != nil {
			http.Error(w, "Failed to add organization member", http.StatusInternalServerError)
			return
		}
		var sendVerificationErr error
		if passwordSetupRequired {
			sendVerificationErr = notifications.SendInvitationEmail(email, verificationToken)
		} else {
			sendVerificationErr = notifications.SendVerificationEmail(email, verificationToken)
		}
		if sendVerificationErr != nil {
			log.Printf("verification email delivery failed for organization member: %v", sendVerificationErr)
			http.Error(w, "Member added, but verification email could not be sent. Retry using the resend endpoint.", http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]string{
			"message": "Member added. They must verify their email before signing in.",
		})
		return
	}

	var currentPermissions []string
	err = tx.QueryRowContext(r.Context(), `
		SELECT permissions FROM shorty.organization_members
		WHERE organization_id = $1 AND user_id = $2`,
		organizationID, memberID,
	).Scan(pq.Array(&currentPermissions))
	if err != nil && err != sql.ErrNoRows {
		http.Error(w, "Failed to read organization membership", http.StatusInternalServerError)
		return
	}
	membershipExists := err == nil
	if !requesterIsOwner && membershipExists && containsPermission(currentPermissions, "organization.manage") &&
		!containsPermission(request.Permissions, "organization.manage") {
		var activeOtherManagers int
		err = tx.QueryRowContext(r.Context(), `
			SELECT COUNT(*)
			FROM shorty.organization_members m
			JOIN shorty.users u ON u.id = m.user_id
			WHERE m.organization_id = $1 AND m.user_id <> $2
			  AND 'organization.manage' = ANY(m.permissions)
			  AND u.deactivated_at IS NULL
			  AND u.email_verified_at IS NOT NULL`,
			organizationID, memberID,
		).Scan(&activeOtherManagers)
		if err != nil {
			http.Error(w, "Failed to verify organization managers", http.StatusInternalServerError)
			return
		}
		if activeOtherManagers == 0 {
			http.Error(w, "Cannot remove the last active organization manager", http.StatusConflict)
			return
		}
	}

	_, err = tx.ExecContext(r.Context(), `
		INSERT INTO shorty.organization_members (organization_id, user_id, permissions)
		VALUES ($1, $2, $3)
		ON CONFLICT (organization_id, user_id) DO UPDATE
		SET permissions = EXCLUDED.permissions`,
		organizationID, memberID, pq.Array(request.Permissions),
	)
	if err != nil {
		if isUniqueViolation(err) {
			http.Error(w, "An account can belong to only one organization", http.StatusConflict)
			return
		}
		http.Error(w, "Failed to add organization member", http.StatusInternalServerError)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "Failed to add organization member", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func isUniqueViolation(err error) bool {
	var postgresError *pq.Error
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}

func ListOrganizationMembers(w http.ResponseWriter, r *http.Request) {
	requesterID, ok := auth.UserID(r)
	if !ok {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	organizationID, ok := organizationIDFromPath(w, r)
	if !ok {
		return
	}
	if !hasOrganizationPermission(w, requesterID, organizationID, "organization.manage") {
		return
	}

	rows, err := db.DB.QueryContext(r.Context(), `
		SELECT u.id, u.email, u.display_name, m.permissions, m.created_at,
		       (o.owner_user_id = u.id)
		FROM shorty.organization_members m
		JOIN shorty.users u ON u.id = m.user_id
		JOIN shorty.organizations o ON o.id = m.organization_id
		WHERE m.organization_id = $1
		ORDER BY u.id`, organizationID)
	if err != nil {
		http.Error(w, "Failed to list organization members", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type member struct {
		UserID      int64     `json:"user_id"`
		Email       string    `json:"email"`
		DisplayName string    `json:"display_name"`
		Permissions []string  `json:"permissions"`
		JoinedAt    time.Time `json:"joined_at"`
		IsOwner     bool      `json:"is_owner"`
	}
	members := make([]member, 0)
	for rows.Next() {
		var item member
		if err := rows.Scan(&item.UserID, &item.Email, &item.DisplayName, pq.Array(&item.Permissions), &item.JoinedAt, &item.IsOwner); err != nil {
			http.Error(w, "Failed to read organization members", http.StatusInternalServerError)
			return
		}
		members = append(members, item)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "Failed to read organization members", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(members); err != nil {
		log.Printf("failed to write organization members response: %v", err)
	}
}

func RemoveOrganizationMember(w http.ResponseWriter, r *http.Request) {
	requesterID, ok := auth.UserID(r)
	if !ok {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	organizationID, ok := organizationIDFromPath(w, r)
	if !ok {
		return
	}
	memberID, err := strconv.ParseInt(r.PathValue("userID"), 10, 64)
	if err != nil || memberID < 1 {
		http.Error(w, "A valid userID is required", http.StatusBadRequest)
		return
	}

	tx, err := db.DB.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "Failed to remove organization member", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var lockedOrganizationID int64
	err = tx.QueryRowContext(r.Context(), `
		SELECT id FROM shorty.organizations WHERE id = $1 FOR UPDATE`,
		organizationID,
	).Scan(&lockedOrganizationID)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Organization not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Failed to verify organization", http.StatusInternalServerError)
		return
	}

	allowed, err := organizationPermissionExists(r.Context(), tx, requesterID, organizationID, "organization.manage")
	if err != nil {
		http.Error(w, "Failed to verify organization permission", http.StatusInternalServerError)
		return
	}
	if !allowed {
		http.Error(w, "Missing organization.manage permission", http.StatusForbidden)
		return
	}
	requesterIsOwner, err := organizationOwnerExists(r.Context(), tx, requesterID, organizationID)
	if err != nil {
		http.Error(w, "Failed to verify organization owner", http.StatusInternalServerError)
		return
	}

	var memberPermissions []string
	err = tx.QueryRowContext(r.Context(), `
		SELECT permissions FROM shorty.organization_members
		WHERE organization_id = $1 AND user_id = $2`,
		organizationID, memberID,
	).Scan(pq.Array(&memberPermissions))
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Organization member not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Failed to read organization member", http.StatusInternalServerError)
		return
	}
	memberIsOwner, err := organizationOwnerExists(r.Context(), tx, memberID, organizationID)
	if err != nil {
		http.Error(w, "Failed to verify organization owner", http.StatusInternalServerError)
		return
	}
	if memberIsOwner {
		http.Error(w, "The organization owner cannot be removed as a member", http.StatusConflict)
		return
	}

	if !requesterIsOwner && containsPermission(memberPermissions, "organization.manage") {
		var activeOtherManagers int
		err = tx.QueryRowContext(r.Context(), `
			SELECT COUNT(*)
			FROM shorty.organization_members m
			JOIN shorty.users u ON u.id = m.user_id
			WHERE m.organization_id = $1 AND m.user_id <> $2
			  AND 'organization.manage' = ANY(m.permissions)
			  AND u.deactivated_at IS NULL
			  AND u.email_verified_at IS NOT NULL`,
			organizationID, memberID,
		).Scan(&activeOtherManagers)
		if err != nil {
			http.Error(w, "Failed to verify organization managers", http.StatusInternalServerError)
			return
		}
		if activeOtherManagers == 0 {
			http.Error(w, "Cannot remove the last active organization manager", http.StatusConflict)
			return
		}
	}

	if _, err = tx.ExecContext(r.Context(), `
		DELETE FROM shorty.organization_members
		WHERE organization_id = $1 AND user_id = $2`,
		organizationID, memberID,
	); err != nil {
		http.Error(w, "Failed to remove organization member", http.StatusInternalServerError)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "Failed to remove organization member", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func organizationIDFromPath(w http.ResponseWriter, r *http.Request) (int64, bool) {
	organizationID, err := strconv.ParseInt(r.PathValue("organizationID"), 10, 64)
	if err != nil || organizationID < 1 {
		http.Error(w, "A valid organizationID is required", http.StatusBadRequest)
		return 0, false
	}
	return organizationID, true
}

func hasOrganizationPermission(w http.ResponseWriter, userID, organizationID int64, permission string) bool {
	var allowed bool
	err := db.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM shorty.organizations
			WHERE id = $1 AND owner_user_id = $2
		) OR EXISTS (
			SELECT 1 FROM shorty.organization_members
			WHERE organization_id = $1 AND user_id = $2
			  AND $3 = ANY(permissions)
		)`, organizationID, userID, permission,
	).Scan(&allowed)
	if err != nil {
		http.Error(w, "Failed to verify organization permission", http.StatusInternalServerError)
		return false
	}
	if !allowed {
		http.Error(w, "Missing organization.manage permission", http.StatusForbidden)
		return false
	}
	return true
}

func organizationPermissionExists(ctx context.Context, tx *sql.Tx, userID, organizationID int64, permission string) (bool, error) {
	var allowed bool
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM shorty.organizations
			WHERE id = $1 AND owner_user_id = $2
		) OR EXISTS (
			SELECT 1 FROM shorty.organization_members
			WHERE organization_id = $1 AND user_id = $2
			  AND $3 = ANY(permissions)
		)`, organizationID, userID, permission,
	).Scan(&allowed)
	return allowed, err
}

func organizationOwnerExists(ctx context.Context, tx *sql.Tx, userID, organizationID int64) (bool, error) {
	var isOwner bool
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM shorty.organizations
			WHERE id = $1 AND owner_user_id = $2
		)`, organizationID, userID,
	).Scan(&isOwner)
	return isOwner, err
}

func containsPermission(permissions []string, permission string) bool {
	return slices.Contains(permissions, permission)
}
