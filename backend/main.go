package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Harsh7213/shorty/db"
	_ "github.com/Harsh7213/shorty/docs"
	"github.com/Harsh7213/shorty/mailqueue"
	"github.com/Harsh7213/shorty/notifications"
	"github.com/Harsh7213/shorty/routes"
)

// @title Shorty
// @version v1.0
// @description URL shortener API.
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter "Bearer" followed by a space and your access token.
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	brokerURL := os.Getenv("RABBITMQ_URL")
	if brokerURL == "" {
		return fmt.Errorf("RABBITMQ_URL must be configured")
	}
	key, err := mailqueue.EncryptionKeyFromEnv()
	if err != nil {
		return err
	}

	if _, err := db.ConnectDB(); err != nil {
		return err
	}
	defer db.CloseDB()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store := mailqueue.SQLStore{DB: db.DB, Key: key}
	background := mailqueue.Start(ctx, store, brokerURL, time.Second, sendQueuedEmail)

	routes.RegisterRoutes()
	server := &http.Server{Addr: ":" + os.Getenv("PORT")}
	serverErr := make(chan error, 1)
	go func() {
		log.Println("Server is running on port", os.Getenv("PORT"))
		serverErr <- server.ListenAndServe()
	}()

	var result error
	select {
	case <-ctx.Done():
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			result = err
		}
		stop() // make the queue goroutines exit too
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil && result == nil {
		result = err
	}
	background.Wait()
	return result
}

func sendQueuedEmail(_ context.Context, payload mailqueue.EmailPayload) error {
	switch payload.Kind {
	case mailqueue.VerificationEmail:
		return notifications.SendVerificationEmail(payload.Recipient, payload.Token)
	case mailqueue.InvitationEmail:
		return notifications.SendInvitationEmail(payload.Recipient, payload.Token)
	default:
		return fmt.Errorf("unsupported email kind %q", payload.Kind)
	}
}
