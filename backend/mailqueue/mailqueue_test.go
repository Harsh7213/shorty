package mailqueue

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	amqp "github.com/rabbitmq/amqp091-go"
)

func TestEncryptAndDecryptEmailPayload(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	payload := []byte(`{"recipient":"person@example.com","token":"verify-me"}`)

	encrypted, err := encrypt(payload, key)
	if err != nil {
		t.Fatalf("encrypt() error = %v", err)
	}
	if reflect.DeepEqual(encrypted, payload) {
		t.Fatal("encrypted payload should not equal plaintext")
	}

	got, err := decrypt(encrypted, key)
	if err != nil {
		t.Fatalf("decrypt() error = %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("decrypt() = %q, want %q", got, payload)
	}
}

func TestDecryptRejectsWrongKeyAndTampering(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	encrypted, err := encrypt([]byte("verification token"), key)
	if err != nil {
		t.Fatalf("encrypt() error = %v", err)
	}

	wrongKey := []byte("abcdefghijklmnopqrstuvwxyzABCDEF")
	if _, err := decrypt(encrypted, wrongKey); err == nil {
		t.Fatal("decrypt() with wrong key should fail")
	}
	encrypted[len(encrypted)-1] ^= 0xff
	if _, err := decrypt(encrypted, key); err == nil {
		t.Fatal("decrypt() with modified ciphertext should fail")
	}
}

func TestEncryptionKeyFromEnv(t *testing.T) {
	t.Setenv("EMAIL_ENCRYPTION_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	key, err := EncryptionKeyFromEnv()
	if err != nil {
		t.Fatalf("EncryptionKeyFromEnv() error = %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("key length = %d, want 32", len(key))
	}
	t.Setenv("EMAIL_ENCRYPTION_KEY", "not-base64")
	if _, err := EncryptionKeyFromEnv(); err == nil {
		t.Fatal("EncryptionKeyFromEnv() should reject invalid base64")
	}
	t.Setenv("EMAIL_ENCRYPTION_KEY", "AA==")
	if _, err := EncryptionKeyFromEnv(); err == nil {
		t.Fatal("EncryptionKeyFromEnv() should reject keys of the wrong length")
	}

	t.Setenv("EMAIL_ENCRYPTION_KEY", "")
	t.Setenv("JWT_SECRET", "abcdefghijklmnopqrstuvwxyzABCDEF")
	derivedKey, err := EncryptionKeyFromEnv()
	if err != nil {
		t.Fatalf("EncryptionKeyFromEnv() fallback error = %v", err)
	}
	if len(derivedKey) != 32 {
		t.Fatalf("derived key length = %d, want 32", len(derivedKey))
	}

	t.Setenv("JWT_SECRET", "short")
	if _, err := EncryptionKeyFromEnv(); err == nil {
		t.Fatal("EncryptionKeyFromEnv() should reject a short fallback secret")
	}
}

func TestEnqueueTxStoresEncryptedPayload(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	key := []byte("01234567890123456789012345678901")
	payload := EmailPayload{
		Kind:      VerificationEmail,
		Recipient: "person@example.com",
		Token:     "verify-me",
		TokenHash: "hashed-token",
	}
	mock.ExpectBegin()
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	mock.ExpectExec("INSERT INTO shorty.email_outbox").
		WithArgs(encryptedPayloadMatcher{key: key, want: payload}).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := EnqueueTx(context.Background(), tx, payload, key); err != nil {
		t.Fatalf("EnqueueTx() error = %v", err)
	}
	mock.ExpectRollback()
	if err := tx.Rollback(); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPublishNextMarksOutboxOnlyAfterSuccessfulPublish(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM shorty.email_outbox").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
	mock.ExpectExec("UPDATE shorty.email_outbox SET published_at").
		WithArgs(int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	publishedID := int64(0)
	published, err := (SQLStore{DB: db}).PublishNext(
		context.Background(),
		func(_ context.Context, id int64) error {
			publishedID = id
			return nil
		},
	)
	if err != nil {
		t.Fatalf("PublishNext() error = %v", err)
	}
	if !published || publishedID != 42 {
		t.Fatalf("PublishNext() = (%v, id %d), want (true, id 42)", published, publishedID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPublishNextRollsBackWhenBrokerPublishFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM shorty.email_outbox").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
	mock.ExpectRollback()

	publishErr := errors.New("broker unavailable")
	published, err := (SQLStore{DB: db}).PublishNext(
		context.Background(),
		func(context.Context, int64) error { return publishErr },
	)
	if !errors.Is(err, publishErr) {
		t.Fatalf("PublishNext() error = %v, want %v", err, publishErr)
	}
	if published {
		t.Fatal("PublishNext() should report no published event")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHandlerRetriesSendFailureAndMarksSuccessfulDelivery(t *testing.T) {
	payload := EmailPayload{
		Kind:      VerificationEmail,
		Recipient: "person@example.com",
		Token:     "verify-me",
		TokenHash: "hashed-token",
	}
	store := &fakeEmailStore{payload: payload}
	sent := 0
	handler := Handler{
		Store: store,
		Sender: func(context.Context, EmailPayload) error {
			sent++
			if sent == 1 {
				return errors.New("SMTP unavailable")
			}
			return nil
		},
	}

	err := handler.Handle(context.Background(), 1)
	if err == nil {
		t.Fatal("first Handle() should fail and be retried")
	}
	if store.delivered != 0 {
		t.Fatal("failed send must not be marked delivered")
	}

	err = handler.Handle(context.Background(), 1)
	if err != nil {
		t.Fatalf("second Handle() error = %v, want success", err)
	}
	if store.delivered != 1 {
		t.Fatalf("MarkDelivered() calls = %d, want 1", store.delivered)
	}
}

func TestHandlerSkipsTerminalJobs(t *testing.T) {
	store := &fakeEmailStore{terminal: true}
	sent := false
	handler := Handler{
		Store: store,
		Sender: func(context.Context, EmailPayload) error {
			sent = true
			return nil
		},
	}

	err := handler.Handle(context.Background(), 1)
	if err != nil {
		t.Fatalf("Handle() error = %v, want terminal success", err)
	}
	if sent {
		t.Fatal("terminal job must not be sent")
	}
}

func TestRetryCount(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]interface{}
		want    int64
	}{
		{name: "no headers", want: 0},
		{
			name: "email queue count",
			headers: map[string]interface{}{
				"x-death": []interface{}{
					amqp.Table{"queue": QueueName, "count": int64(3)},
				},
			},
			want: 3,
		},
		{
			name: "unrelated queue",
			headers: map[string]interface{}{
				"x-death": []interface{}{
					amqp.Table{"queue": "another.queue", "count": int64(4)},
				},
			},
		},
		{
			name: "explicit retry header",
			headers: map[string]interface{}{
				"x-shorty-retry": int32(2),
			},
			want: 2,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := RetryCount(test.headers); got != test.want {
				t.Fatalf("RetryCount() = %d, want %d", got, test.want)
			}
		})
	}
}

type fakeEmailStore struct {
	payload   EmailPayload
	terminal  bool
	delivered int
}

type encryptedPayloadMatcher struct {
	key  []byte
	want EmailPayload
}

func (m encryptedPayloadMatcher) Match(value driver.Value) bool {
	ciphertext, ok := value.([]byte)
	if !ok {
		return false
	}
	plaintext, err := decrypt(ciphertext, m.key)
	if err != nil {
		return false
	}
	var got EmailPayload
	if err := json.Unmarshal(plaintext, &got); err != nil {
		return false
	}
	return reflect.DeepEqual(got, m.want)
}

func (s *fakeEmailStore) LoadEmail(context.Context, int64) (EmailPayload, bool, error) {
	return s.payload, s.terminal, nil
}

func (s *fakeEmailStore) MarkDelivered(context.Context, int64) error {
	s.delivered++
	return nil
}

func (*fakeEmailStore) MarkFailed(context.Context, int64, string) error {
	return nil
}
