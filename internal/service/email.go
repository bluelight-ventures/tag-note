package service

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

// EmailSender abstracts the low-level "send an email" mechanism.
type EmailSender interface {
	SendRawEmail(from string, to string, subject string, body string) error
}

// EmailService handles sending verification and password reset emails.
type EmailService struct {
	sender    EmailSender
	fromEmail string
	baseURL   string
	enabled   bool
}

// NewEmailService creates a new EmailService configured from environment variables.
// Priority: 1) Amazon SES, 2) SMTP, 3) sendmail (opt-in).
func NewEmailService() *EmailService {
	fromEmail := os.Getenv("EMAIL_FROM")
	if fromEmail == "" {
		fromEmail = "noreply@example.com"
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:3000"
	}

	var sender EmailSender
	var enabled bool

	// Priority 1: Amazon SES
	if accessKey := os.Getenv("AWS_SES_ACCESS_KEY"); accessKey != "" {
		secretKey := os.Getenv("AWS_SES_SECRET_KEY")
		region := os.Getenv("AWS_SES_REGION")
		if region == "" {
			region = "us-east-1"
		}
		ses, err := NewSESSender(accessKey, secretKey, region)
		if err == nil {
			sender = ses
			enabled = true
		}
	}

	// Priority 2: SMTP
	if sender == nil {
		if host := os.Getenv("SMTP_HOST"); host != "" {
			port, _ := strconv.Atoi(os.Getenv("SMTP_PORT"))
			if port == 0 {
				port = 587
			}
			sender = &SMTPSender{
				Host:     host,
				Port:     port,
				User:     os.Getenv("SMTP_USER"),
				Password: os.Getenv("SMTP_PASSWORD"),
			}
			enabled = true
		}
	}

	// Priority 3: sendmail (opt-in)
	if sender == nil && os.Getenv("USE_SENDMAIL") == "1" {
		if _, err := exec.LookPath("sendmail"); err == nil {
			sender = &SendmailSender{}
			enabled = true
		}
	}

	return &EmailService{
		sender:    sender,
		fromEmail: fromEmail,
		baseURL:   baseURL,
		enabled:   enabled,
	}
}

// IsEnabled returns whether the email service is configured.
func (e *EmailService) IsEnabled() bool {
	return e.enabled
}

// SendVerificationEmail sends an email verification link to the user.
func (e *EmailService) SendVerificationEmail(to, token string) error {
	if !e.enabled {
		return nil
	}

	subject := "Verify your TagNote email address"
	verifyURL := fmt.Sprintf("%s/app?verify=%s", e.baseURL, token)

	body := fmt.Sprintf(`Hello,

Please verify your email address by clicking the link below:

%s

This link will expire in 24 hours.

If you didn't create a TagNote account, you can ignore this email.

Thanks,
The TagNote Team
`, verifyURL)

	return e.sendMail(to, subject, body)
}

// SendPasswordResetEmail sends a password reset link to the user.
func (e *EmailService) SendPasswordResetEmail(to, token string) error {
	if !e.enabled {
		return nil
	}

	subject := "Reset your TagNote password"
	resetURL := fmt.Sprintf("%s/app?reset=%s", e.baseURL, token)

	body := fmt.Sprintf(`Hello,

You requested to reset your TagNote password. Click the link below to set a new password:

%s

This link will expire in 1 hour.

If you didn't request a password reset, you can ignore this email.

Thanks,
The TagNote Team
`, resetURL)

	return e.sendMail(to, subject, body)
}

// SendMagicLinkEmail sends a magic link for passwordless login.
func (e *EmailService) SendMagicLinkEmail(to, token string) error {
	if !e.enabled {
		return nil
	}

	subject := "Your TagNote login link"
	loginURL := fmt.Sprintf("%s/app?magic=%s", e.baseURL, token)

	body := fmt.Sprintf(`Hello,

Click the link below to log in to TagNote:

%s

This link will expire in 15 minutes and can only be used once.

If you didn't request this login link, you can ignore this email.

Thanks,
The TagNote Team
`, loginURL)

	return e.sendMail(to, subject, body)
}

func (e *EmailService) sendMail(to, subject, body string) error {
	if e.sender == nil {
		return fmt.Errorf("no email sender configured")
	}
	return e.sender.SendRawEmail(e.fromEmail, to, subject, body)
}
