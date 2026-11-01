package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

func GenerateShortURLCode(longURL string) (string, error) {
	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	input := longURL + hex.EncodeToString(randomBytes)
	hash := sha256.Sum256([]byte(input))

	shortURL := hex.EncodeToString(hash[:])[:10]

	return shortURL, nil
}
