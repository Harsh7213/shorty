package notifications

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"net/url"
	"os"
	"strings"
)

func SendVerificationEmail(email, token string) error {
	apiURL, err := verificationAPIURL()
	if err != nil {
		return err
	}
	verifyURL := apiURL + "/api/v1/auth/verify?token=" + url.QueryEscape(token)
	return sendEmail(email, "Verify your Shorty account", strings.Join([]string{
		"Verify your Shorty account by opening this link:",
		verifyURL,
		"",
		"This link expires in 24 hours. If you did not create this account, ignore this email.",
	}, "\r\n"))
}

func SendInvitationEmail(email, token string) error {
	apiURL, err := verificationAPIURL()
	if err != nil {
		return err
	}
	verifyURL := apiURL + "/api/v1/auth/verify?token=" + url.QueryEscape(token)
	return sendEmail(email, "You have been invited to Shorty", strings.Join([]string{
		"An organization owner invited you to Shorty.",
		"Verify your email, then sign in with the temporary password provided by your organization administrator.",
		verifyURL,
		"",
		"After signing in, change your password from your account settings.",
		"",
		"This invitation expires in 24 hours. If you did not expect this invitation, ignore this email.",
	}, "\r\n"))
}

func verificationAPIURL() (string, error) {
	apiURL := strings.TrimRight(os.Getenv("PUBLIC_API_URL"), "/")
	if apiURL == "" {
		return "", fmt.Errorf("PUBLIC_API_URL must be configured")
	}
	publicURL, err := url.Parse(apiURL)
	if err != nil || (publicURL.Scheme != "https" && publicURL.Scheme != "http") || publicURL.Host == "" {
		return "", fmt.Errorf("PUBLIC_API_URL must be an absolute HTTP or HTTPS URL")
	}
	return apiURL, nil
}

func sendEmail(email, subject, bodyText string) error {
	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")
	from := os.Getenv("SMTP_FROM")
	if host == "" || port == "" || from == "" {
		return fmt.Errorf("SMTP_HOST, SMTP_PORT, and SMTP_FROM must be configured")
	}

	message := strings.Join([]string{
		"To: " + email,
		"From: " + from,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		bodyText,
	}, "\r\n")

	client, err := smtp.Dial(net.JoinHostPort(host, port))
	if err != nil {
		return fmt.Errorf("connect to SMTP server: %w", err)
	}
	defer client.Close()

	useSTARTTLS, err := smtpSTARTTLSRequired(host, os.Getenv("SMTP_STARTTLS"))
	if err != nil {
		return err
	}
	if useSTARTTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("SMTP server does not support STARTTLS")
		}
		if err := client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("start SMTP TLS: %w", err)
		}
	}

	username := os.Getenv("SMTP_USERNAME")
	if username != "" {
		if err := client.Auth(smtp.PlainAuth("", username, os.Getenv("SMTP_PASSWORD"), host)); err != nil {
			return fmt.Errorf("authenticate with SMTP server: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("set SMTP sender: %w", err)
	}
	if err := client.Rcpt(email); err != nil {
		return fmt.Errorf("set SMTP recipient: %w", err)
	}
	body, err := client.Data()
	if err != nil {
		return fmt.Errorf("start SMTP message: %w", err)
	}
	if _, err := body.Write([]byte(message)); err != nil {
		_ = body.Close()
		return fmt.Errorf("write SMTP message: %w", err)
	}
	if err := body.Close(); err != nil {
		return fmt.Errorf("send SMTP message: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("close SMTP session: %w", err)
	}
	return nil
}

func smtpSTARTTLSRequired(host, setting string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(setting)) {
	case "", "true":
		return true, nil
	case "false":
		ip := net.ParseIP(strings.Trim(host, "[]"))
		if strings.EqualFold(host, "localhost") || (ip != nil && ip.IsLoopback()) {
			return false, nil
		}
		return false, fmt.Errorf("SMTP_STARTTLS=false is only allowed for localhost or a loopback IP")
	default:
		return false, fmt.Errorf("SMTP_STARTTLS must be true or false")
	}
}
