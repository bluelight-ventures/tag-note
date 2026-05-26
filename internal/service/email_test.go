package service

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// mockSender is a test double for EmailSender that records calls and can simulate errors.
type mockSender struct {
	calls     []mockSendCall
	returnErr error
}

type mockSendCall struct {
	from    string
	to      string
	subject string
	body    string
}

func (m *mockSender) SendRawEmail(from, to, subject, body string) error {
	m.calls = append(m.calls, mockSendCall{from, to, subject, body})
	return m.returnErr
}

func (m *mockSender) lastCall() *mockSendCall {
	if len(m.calls) == 0 {
		return nil
	}
	return &m.calls[len(m.calls)-1]
}

func (m *mockSender) callCount() int {
	return len(m.calls)
}

// newTestEmailService creates an EmailService with a mock sender for testing.
func newTestEmailService(sender EmailSender, fromEmail, baseURL string, enabled bool) *EmailService {
	return &EmailService{
		sender:    sender,
		fromEmail: fromEmail,
		baseURL:   baseURL,
		enabled:   enabled,
	}
}

func TestEmailService_IsEnabled(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		want    bool
	}{
		{"enabled service returns true", true, true},
		{"disabled service returns false", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestEmailService(&mockSender{}, "test@example.com", "http://localhost", tt.enabled)
			if got := svc.IsEnabled(); got != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEmailService_SendVerificationEmail(t *testing.T) {
	tests := []struct {
		name          string
		enabled       bool
		senderErr     error
		to            string
		token         string
		wantCalls     int
		wantErr       bool
		wantSubject   string
		wantURLInBody string
	}{
		{
			name:          "sends verification email when enabled",
			enabled:       true,
			to:            "user@example.com",
			token:         "abc123",
			wantCalls:     1,
			wantErr:       false,
			wantSubject:   "Verify your TagNote email address",
			wantURLInBody: "http://test.com/app?verify=abc123",
		},
		{
			name:      "skips sending when disabled",
			enabled:   false,
			to:        "user@example.com",
			token:     "abc123",
			wantCalls: 0,
			wantErr:   false,
		},
		{
			name:      "returns sender error",
			enabled:   true,
			senderErr: errors.New("SMTP connection failed"),
			to:        "user@example.com",
			token:     "xyz789",
			wantCalls: 1,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockSender{returnErr: tt.senderErr}
			svc := newTestEmailService(mock, "noreply@example.com", "http://test.com", tt.enabled)

			err := svc.SendVerificationEmail(tt.to, tt.token)

			if (err != nil) != tt.wantErr {
				t.Errorf("SendVerificationEmail() error = %v, wantErr %v", err, tt.wantErr)
			}

			if mock.callCount() != tt.wantCalls {
				t.Errorf("SendVerificationEmail() made %d calls, want %d", mock.callCount(), tt.wantCalls)
			}

			if tt.wantCalls > 0 {
				call := mock.lastCall()
				if call.to != tt.to {
					t.Errorf("SendVerificationEmail() sent to %q, want %q", call.to, tt.to)
				}
				if call.from != "noreply@example.com" {
					t.Errorf("SendVerificationEmail() from %q, want %q", call.from, "noreply@example.com")
				}
				if tt.wantSubject != "" && call.subject != tt.wantSubject {
					t.Errorf("SendVerificationEmail() subject = %q, want %q", call.subject, tt.wantSubject)
				}
				if tt.wantURLInBody != "" && !strings.Contains(call.body, tt.wantURLInBody) {
					t.Errorf("SendVerificationEmail() body missing URL %q", tt.wantURLInBody)
				}
			}
		})
	}
}

func TestEmailService_SendPasswordResetEmail(t *testing.T) {
	tests := []struct {
		name          string
		enabled       bool
		senderErr     error
		to            string
		token         string
		wantCalls     int
		wantErr       bool
		wantSubject   string
		wantURLInBody string
	}{
		{
			name:          "sends reset email when enabled",
			enabled:       true,
			to:            "user@example.com",
			token:         "reset456",
			wantCalls:     1,
			wantErr:       false,
			wantSubject:   "Reset your TagNote password",
			wantURLInBody: "http://test.com/app?reset=reset456",
		},
		{
			name:      "skips sending when disabled",
			enabled:   false,
			to:        "user@example.com",
			token:     "reset456",
			wantCalls: 0,
			wantErr:   false,
		},
		{
			name:      "returns sender error",
			enabled:   true,
			senderErr: errors.New("network timeout"),
			to:        "user@example.com",
			token:     "reset456",
			wantCalls: 1,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockSender{returnErr: tt.senderErr}
			svc := newTestEmailService(mock, "noreply@example.com", "http://test.com", tt.enabled)

			err := svc.SendPasswordResetEmail(tt.to, tt.token)

			if (err != nil) != tt.wantErr {
				t.Errorf("SendPasswordResetEmail() error = %v, wantErr %v", err, tt.wantErr)
			}

			if mock.callCount() != tt.wantCalls {
				t.Errorf("SendPasswordResetEmail() made %d calls, want %d", mock.callCount(), tt.wantCalls)
			}

			if tt.wantCalls > 0 {
				call := mock.lastCall()
				if call.to != tt.to {
					t.Errorf("SendPasswordResetEmail() sent to %q, want %q", call.to, tt.to)
				}
				if tt.wantSubject != "" && call.subject != tt.wantSubject {
					t.Errorf("SendPasswordResetEmail() subject = %q, want %q", call.subject, tt.wantSubject)
				}
				if tt.wantURLInBody != "" && !strings.Contains(call.body, tt.wantURLInBody) {
					t.Errorf("SendPasswordResetEmail() body missing URL %q", tt.wantURLInBody)
				}
			}
		})
	}
}

func TestEmailService_sendMail_NilSender(t *testing.T) {
	svc := &EmailService{
		sender:    nil,
		fromEmail: "test@example.com",
		baseURL:   "http://localhost",
		enabled:   true,
	}

	err := svc.sendMail("to@example.com", "Subject", "Body")
	if err == nil {
		t.Error("sendMail() with nil sender should return error")
	}
	if !strings.Contains(err.Error(), "no email sender configured") {
		t.Errorf("sendMail() error = %q, want error containing 'no email sender configured'", err.Error())
	}
}

func TestNewEmailService_Defaults(t *testing.T) {
	// Clear all email-related env vars
	envVars := []string{
		"EMAIL_FROM", "BASE_URL", "AWS_SES_ACCESS_KEY", "AWS_SES_SECRET_KEY",
		"AWS_SES_REGION", "SMTP_HOST", "SMTP_PORT", "SMTP_USER", "SMTP_PASSWORD",
		"USE_SENDMAIL",
	}
	original := make(map[string]string)
	for _, v := range envVars {
		original[v] = os.Getenv(v)
		os.Unsetenv(v)
	}
	defer func() {
		for k, v := range original {
			if v != "" {
				os.Setenv(k, v)
			}
		}
	}()

	svc := NewEmailService()

	// With no config, service should be disabled
	if svc.IsEnabled() {
		t.Error("NewEmailService() should be disabled when no sender is configured")
	}

	// Check default values are set
	if svc.fromEmail != "noreply@example.com" {
		t.Errorf("NewEmailService() fromEmail = %q, want %q", svc.fromEmail, "noreply@example.com")
	}
	if svc.baseURL != "http://localhost:3000" {
		t.Errorf("NewEmailService() baseURL = %q, want %q", svc.baseURL, "http://localhost:3000")
	}
}

func TestNewEmailService_CustomFromAndBaseURL(t *testing.T) {
	// Clear env vars first
	envVars := []string{
		"EMAIL_FROM", "BASE_URL", "AWS_SES_ACCESS_KEY", "AWS_SES_SECRET_KEY",
		"AWS_SES_REGION", "SMTP_HOST", "USE_SENDMAIL",
	}
	original := make(map[string]string)
	for _, v := range envVars {
		original[v] = os.Getenv(v)
		os.Unsetenv(v)
	}
	defer func() {
		for k, v := range original {
			if v != "" {
				os.Setenv(k, v)
			} else {
				os.Unsetenv(k)
			}
		}
	}()

	os.Setenv("EMAIL_FROM", "custom@example.com")
	os.Setenv("BASE_URL", "https://notes.example.com")

	svc := NewEmailService()

	if svc.fromEmail != "custom@example.com" {
		t.Errorf("NewEmailService() fromEmail = %q, want %q", svc.fromEmail, "custom@example.com")
	}
	if svc.baseURL != "https://notes.example.com" {
		t.Errorf("NewEmailService() baseURL = %q, want %q", svc.baseURL, "https://notes.example.com")
	}
}

func TestNewEmailService_SMTPConfiguration(t *testing.T) {
	// Clear env vars first
	envVars := []string{
		"EMAIL_FROM", "BASE_URL", "AWS_SES_ACCESS_KEY", "AWS_SES_SECRET_KEY",
		"AWS_SES_REGION", "SMTP_HOST", "SMTP_PORT", "SMTP_USER", "SMTP_PASSWORD",
		"USE_SENDMAIL",
	}
	original := make(map[string]string)
	for _, v := range envVars {
		original[v] = os.Getenv(v)
		os.Unsetenv(v)
	}
	defer func() {
		for k, v := range original {
			if v != "" {
				os.Setenv(k, v)
			} else {
				os.Unsetenv(k)
			}
		}
	}()

	os.Setenv("SMTP_HOST", "smtp.example.com")
	os.Setenv("SMTP_PORT", "465")
	os.Setenv("SMTP_USER", "smtpuser")
	os.Setenv("SMTP_PASSWORD", "smtppass")

	svc := NewEmailService()

	if !svc.IsEnabled() {
		t.Error("NewEmailService() should be enabled with SMTP config")
	}

	// Verify it's an SMTP sender
	smtpSender, ok := svc.sender.(*SMTPSender)
	if !ok {
		t.Fatalf("NewEmailService() sender type = %T, want *SMTPSender", svc.sender)
	}
	if smtpSender.Host != "smtp.example.com" {
		t.Errorf("SMTPSender.Host = %q, want %q", smtpSender.Host, "smtp.example.com")
	}
	if smtpSender.Port != 465 {
		t.Errorf("SMTPSender.Port = %d, want %d", smtpSender.Port, 465)
	}
	if smtpSender.User != "smtpuser" {
		t.Errorf("SMTPSender.User = %q, want %q", smtpSender.User, "smtpuser")
	}
	if smtpSender.Password != "smtppass" {
		t.Errorf("SMTPSender.Password = %q, want %q", smtpSender.Password, "smtppass")
	}
}

func TestNewEmailService_SMTPDefaultPort(t *testing.T) {
	envVars := []string{
		"AWS_SES_ACCESS_KEY", "SMTP_HOST", "SMTP_PORT", "USE_SENDMAIL",
	}
	original := make(map[string]string)
	for _, v := range envVars {
		original[v] = os.Getenv(v)
		os.Unsetenv(v)
	}
	defer func() {
		for k, v := range original {
			if v != "" {
				os.Setenv(k, v)
			} else {
				os.Unsetenv(k)
			}
		}
	}()

	os.Setenv("SMTP_HOST", "smtp.example.com")
	// Don't set SMTP_PORT - should default to 587

	svc := NewEmailService()

	smtpSender, ok := svc.sender.(*SMTPSender)
	if !ok {
		t.Fatalf("NewEmailService() sender type = %T, want *SMTPSender", svc.sender)
	}
	if smtpSender.Port != 587 {
		t.Errorf("SMTPSender.Port = %d, want default %d", smtpSender.Port, 587)
	}
}

func TestNewEmailService_SESConfiguration(t *testing.T) {
	envVars := []string{
		"AWS_SES_ACCESS_KEY", "AWS_SES_SECRET_KEY", "AWS_SES_REGION",
		"SMTP_HOST", "USE_SENDMAIL",
	}
	original := make(map[string]string)
	for _, v := range envVars {
		original[v] = os.Getenv(v)
		os.Unsetenv(v)
	}
	defer func() {
		for k, v := range original {
			if v != "" {
				os.Setenv(k, v)
			} else {
				os.Unsetenv(k)
			}
		}
	}()

	os.Setenv("AWS_SES_ACCESS_KEY", "TEST_AWS_ACCESS_KEY")
	os.Setenv("AWS_SES_SECRET_KEY", "TEST_AWS_SECRET_KEY")
	os.Setenv("AWS_SES_REGION", "eu-west-1")

	svc := NewEmailService()

	if !svc.IsEnabled() {
		t.Error("NewEmailService() should be enabled with SES config")
	}

	// Verify it's a SES sender
	_, ok := svc.sender.(*SESSender)
	if !ok {
		t.Errorf("NewEmailService() sender type = %T, want *SESSender", svc.sender)
	}
}

func TestNewEmailService_SESPriorityOverSMTP(t *testing.T) {
	envVars := []string{
		"AWS_SES_ACCESS_KEY", "AWS_SES_SECRET_KEY", "AWS_SES_REGION",
		"SMTP_HOST", "USE_SENDMAIL",
	}
	original := make(map[string]string)
	for _, v := range envVars {
		original[v] = os.Getenv(v)
		os.Unsetenv(v)
	}
	defer func() {
		for k, v := range original {
			if v != "" {
				os.Setenv(k, v)
			} else {
				os.Unsetenv(k)
			}
		}
	}()

	// Configure both SES and SMTP
	os.Setenv("AWS_SES_ACCESS_KEY", "TEST_AWS_ACCESS_KEY")
	os.Setenv("AWS_SES_SECRET_KEY", "TEST_AWS_SECRET_KEY")
	os.Setenv("SMTP_HOST", "smtp.example.com")

	svc := NewEmailService()

	// SES should take priority
	_, ok := svc.sender.(*SESSender)
	if !ok {
		t.Errorf("NewEmailService() with both SES and SMTP configured should use SES, got %T", svc.sender)
	}
}

func TestNewEmailService_SESDefaultRegion(t *testing.T) {
	envVars := []string{
		"AWS_SES_ACCESS_KEY", "AWS_SES_SECRET_KEY", "AWS_SES_REGION",
		"SMTP_HOST", "USE_SENDMAIL",
	}
	original := make(map[string]string)
	for _, v := range envVars {
		original[v] = os.Getenv(v)
		os.Unsetenv(v)
	}
	defer func() {
		for k, v := range original {
			if v != "" {
				os.Setenv(k, v)
			} else {
				os.Unsetenv(k)
			}
		}
	}()

	os.Setenv("AWS_SES_ACCESS_KEY", "TEST_AWS_ACCESS_KEY")
	os.Setenv("AWS_SES_SECRET_KEY", "TEST_AWS_SECRET_KEY")
	// Don't set AWS_SES_REGION - should default to us-east-1

	svc := NewEmailService()

	if !svc.IsEnabled() {
		t.Error("NewEmailService() should be enabled with SES config (default region)")
	}
	_, ok := svc.sender.(*SESSender)
	if !ok {
		t.Errorf("NewEmailService() sender type = %T, want *SESSender", svc.sender)
	}
}
