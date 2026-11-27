package mailqueue

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	QueueName        = "shorty.email"
	RetryQueuePrefix = "shorty.email.retry"
	DeadQueueName    = "shorty.email.dead"
	MaxDeliveryRuns  = int64(5)
)

var retryDelays = []time.Duration{
	30 * time.Second,
	time.Minute,
	2 * time.Minute,
	4 * time.Minute,
}

type EmailKind string

const (
	VerificationEmail EmailKind = "verification"
	InvitationEmail   EmailKind = "invitation"
)

type EmailPayload struct {
	Kind      EmailKind `json:"kind"`
	Recipient string    `json:"recipient"`
	Token     string    `json:"token"`
	TokenHash string    `json:"token_hash"`
}

type Event struct {
	ID int64 `json:"id"`
}

type EmailStore interface {
	LoadEmail(context.Context, int64) (EmailPayload, bool, error)
	MarkDelivered(context.Context, int64) error
	MarkFailed(context.Context, int64, string) error
}

type Sender func(context.Context, EmailPayload) error

type Handler struct {
	Store  EmailStore
	Sender Sender
}

func EncryptionKeyFromEnv() ([]byte, error) {
	value := strings.TrimSpace(os.Getenv("EMAIL_ENCRYPTION_KEY"))
	if value == "" {
		secret := os.Getenv("JWT_SECRET")
		if len(secret) < 32 {
			return nil, fmt.Errorf("EMAIL_ENCRYPTION_KEY must be configured or JWT_SECRET must be at least 32 bytes")
		}
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write([]byte("shorty/email-outbox-encryption/v1"))
		return mac.Sum(nil), nil
	}
	key, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("EMAIL_ENCRYPTION_KEY must be base64-encoded: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("EMAIL_ENCRYPTION_KEY must decode to exactly 32 bytes")
	}
	return key, nil
}

func EnqueueTx(ctx context.Context, tx *sql.Tx, payload EmailPayload, key []byte) error {
	plaintext, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode email payload: %w", err)
	}
	encrypted, err := encrypt(plaintext, key)
	if err != nil {
		return fmt.Errorf("encrypt email payload: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO shorty.email_outbox (encrypted_payload)
		VALUES ($1)`, encrypted)
	if err != nil {
		return fmt.Errorf("insert email outbox job: %w", err)
	}
	return nil
}

func encrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate email payload nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func decrypt(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, fmt.Errorf("encrypted email payload is truncated")
	}
	nonce, body := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt email payload: %w", err)
	}
	return plaintext, nil
}

func (h Handler) Handle(ctx context.Context, eventID int64) error {
	payload, terminal, err := h.Store.LoadEmail(ctx, eventID)
	if err != nil {
		return err
	}
	if terminal {
		return nil
	}
	if err := h.Sender(ctx, payload); err != nil {
		return err
	}
	if err := h.Store.MarkDelivered(ctx, eventID); err != nil {
		return fmt.Errorf("mark email job delivered: %w", err)
	}
	return nil
}

type SQLStore struct {
	DB  *sql.DB
	Key []byte
}

func (s SQLStore) PublishNext(ctx context.Context, publish func(context.Context, int64) error) (bool, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin outbox transaction: %w", err)
	}
	defer tx.Rollback()

	var eventID int64
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM shorty.email_outbox
		WHERE published_at IS NULL AND failed_at IS NULL
		ORDER BY id
		LIMIT 1
		FOR UPDATE SKIP LOCKED`).Scan(&eventID)
	if err == sql.ErrNoRows {
		if err := tx.Commit(); err != nil {
			return false, fmt.Errorf("commit empty outbox transaction: %w", err)
		}
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("load next email outbox job: %w", err)
	}
	if err := publish(ctx, eventID); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE shorty.email_outbox SET published_at = NOW() WHERE id = $1`, eventID); err != nil {
		return false, fmt.Errorf("mark email outbox job published: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit published email job: %w", err)
	}
	return true, nil
}

func (s SQLStore) LoadEmail(ctx context.Context, eventID int64) (EmailPayload, bool, error) {
	var encrypted []byte
	var deliveredAt, failedAt sql.NullTime
	err := s.DB.QueryRowContext(ctx, `
		SELECT encrypted_payload, delivered_at, failed_at
		FROM shorty.email_outbox WHERE id = $1`, eventID,
	).Scan(&encrypted, &deliveredAt, &failedAt)
	if err != nil {
		return EmailPayload{}, false, fmt.Errorf("load email job %d: %w", eventID, err)
	}
	if deliveredAt.Valid || failedAt.Valid {
		return EmailPayload{}, true, nil
	}
	plaintext, err := decrypt(encrypted, s.Key)
	if err != nil {
		return EmailPayload{}, false, err
	}
	var payload EmailPayload
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		return EmailPayload{}, false, fmt.Errorf("decode email payload: %w", err)
	}
	var tokenIsCurrent bool
	err = s.DB.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM shorty.email_verification_tokens
			WHERE token_hash = $1 AND expires_at > NOW()
		)`, payload.TokenHash).Scan(&tokenIsCurrent)
	if err != nil {
		return EmailPayload{}, false, fmt.Errorf("check verification token: %w", err)
	}
	if !tokenIsCurrent {
		if err := s.MarkFailed(ctx, eventID, "verification token expired or superseded"); err != nil {
			return EmailPayload{}, false, err
		}
		return EmailPayload{}, true, nil
	}
	return payload, false, nil
}

func (s SQLStore) MarkDelivered(ctx context.Context, eventID int64) error {
	result, err := s.DB.ExecContext(ctx, `
		UPDATE shorty.email_outbox
		SET delivered_at = NOW(), encrypted_payload = NULL, last_error = NULL
		WHERE id = $1 AND delivered_at IS NULL AND failed_at IS NULL`, eventID)
	if err != nil {
		return fmt.Errorf("update delivered email job: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check delivered email job rows: %w", err)
	}
	if rows != 1 {
		return fmt.Errorf("email job %d was not marked delivered", eventID)
	}
	return nil
}

func (s SQLStore) MarkFailed(ctx context.Context, eventID int64, reason string) error {
	_, err := s.DB.ExecContext(ctx, `
		UPDATE shorty.email_outbox
		SET failed_at = NOW(), last_error = $2
		WHERE id = $1 AND delivered_at IS NULL AND failed_at IS NULL`, eventID, reason)
	if err != nil {
		return fmt.Errorf("mark email job failed: %w", err)
	}
	return nil
}

func (s SQLStore) RetryFailed(ctx context.Context, eventID int64) error {
	result, err := s.DB.ExecContext(ctx, `
		UPDATE shorty.email_outbox
		SET failed_at = NULL, published_at = NULL, last_error = NULL
		WHERE id = $1 AND delivered_at IS NULL AND published_at IS NOT NULL
		  AND encrypted_payload IS NOT NULL`, eventID)
	if err != nil {
		return fmt.Errorf("reset failed email job: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check retried email job rows: %w", err)
	}
	if rows != 1 {
		return fmt.Errorf("email job %d is not retryable", eventID)
	}
	return nil
}

func RetryCount(headers map[string]interface{}) int64 {
	switch count := headers["x-shorty-retry"].(type) {
	case int64:
		return count
	case int32:
		return int64(count)
	case int:
		return int64(count)
	}
	deaths, ok := headers["x-death"].([]interface{})
	if !ok {
		return 0
	}
	for _, entry := range deaths {
		death, ok := entry.(amqp.Table)
		if !ok {
			death, ok = entry.(map[string]interface{})
		}
		if !ok || death["queue"] != QueueName {
			continue
		}
		switch count := death["count"].(type) {
		case int64:
			return count
		case int32:
			return int64(count)
		}
	}
	return 0
}

func retryQueueName(retryCount int64) string {
	return fmt.Sprintf("%s.%d", RetryQueuePrefix, retryCount+1)
}

func logDeliveryError(action string, err error) {
	if err != nil {
		log.Printf("email queue %s: %v", action, err)
	}
}

func Wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
