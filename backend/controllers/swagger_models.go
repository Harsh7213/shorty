package controllers

import (
	"encoding/json"
	"time"
)

type RegisterUserRequest struct {
	Email       string `json:"email" example:"person@example.com"`
	Password    string `json:"password" example:"at-least-12-characters"`
	DisplayName string `json:"display_name" example:"Person"`
}

type LoginRequest struct {
	Email    string `json:"email" example:"person@example.com"`
	Password string `json:"password" example:"your-password"`
}

type ResendVerificationRequest struct {
	Email string `json:"email" example:"person@example.com"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" example:"current-password"`
	NewPassword     string `json:"new_password" example:"a-new-password"`
}

type CreateOrganizationRequest struct {
	Name string `json:"name" example:"Example"`
}

type AddOrganizationMemberRequest struct {
	Email       string   `json:"email" example:"person@example.com"`
	DisplayName string   `json:"display_name" example:"Person"`
	Password    string   `json:"password,omitempty" example:"temporary-password"`
	Permissions []string `json:"permissions" example:"link.manage"`
}

type CreateLinkRequest struct {
	OrganizationID int64      `json:"organization_id" example:"1"`
	LongURL        string     `json:"long_url" example:"https://example.com"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty" swaggertype:"string" example:"2027-01-01T00:00:00Z"`
}

type UpdateLinkRequest struct {
	Status    *string         `json:"status,omitempty" enums:"active,disabled,revoked" example:"disabled"`
	ExpiresAt json.RawMessage `json:"expires_at,omitempty" swaggertype:"string" example:"2027-01-01T00:00:00Z"`
}
