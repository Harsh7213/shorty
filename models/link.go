package models

import "time"

type Link struct {
	ID           int64     `json:"id"`
	LongURL      string    `json:"long_url"`
	ShortURLCode string    `json:"-"`
	ShortURL     string    `json:"short_url,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type CreateLinkResponse struct {
	ShortURL string `json:"short_url"`
}
