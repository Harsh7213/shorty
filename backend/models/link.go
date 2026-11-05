package models

import "time"

type Link struct {
	ID              int64      `json:"id"`
	OrganizationID  int64      `json:"-"`
	CreatedByUserID int64      `json:"-"`
	LongURL         string     `json:"long_url"`
	ShortURLCode    string     `json:"-"`
	ShortURL        string     `json:"short_url,omitempty"`
	Status          string     `json:"-"`
	RevokedAt       *time.Time `json:"-"`
	ExpiresAt       *time.Time `json:"-"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"-"`
}

type Organization struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	OwnerUserID int64     `json:"owner_user_id"`
	CreatedAt   time.Time `json:"created_at"`
}

type OrganizationMember struct {
	OrganizationID int64     `json:"organization_id"`
	UserID         int64     `json:"user_id"`
	Permissions    []string  `json:"permissions"`
	CreatedAt      time.Time `json:"created_at"`
}

type CreateLinkResponse struct {
	ShortURL string `json:"short_url"`
}
