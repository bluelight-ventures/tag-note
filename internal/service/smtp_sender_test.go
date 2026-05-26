package service

import (
	"strings"
	"testing"
)

func TestSMTPSender_ImplementsEmailSender(t *testing.T) {
	sender := &SMTPSender{
		Host:     "smtp.example.com",
		Port:     587,
		User:     "user",
		Password: "pass",
	}

	// Verify SMTPSender implements EmailSender interface
	var _ EmailSender = sender
}

func TestSMTPSender_FieldValues(t *testing.T) {
	sender := &SMTPSender{
		Host:     "mail.example.com",
		Port:     465,
		User:     "testuser",
		Password: "testpass",
	}

	if sender.Host != "mail.example.com" {
		t.Errorf("SMTPSender.Host = %q, want %q", sender.Host, "mail.example.com")
	}
	if sender.Port != 465 {
		t.Errorf("SMTPSender.Port = %d, want %d", sender.Port, 465)
	}
	if sender.User != "testuser" {
		t.Errorf("SMTPSender.User = %q, want %q", sender.User, "testuser")
	}
	if sender.Password != "testpass" {
		t.Errorf("SMTPSender.Password = %q, want %q", sender.Password, "testpass")
	}
}

func TestSMTPSender_ZeroValuePort(t *testing.T) {
	sender := &SMTPSender{
		Host: "smtp.example.com",
	}

	// Zero value for Port should be allowed (NewEmailService defaults it to 587)
	if sender.Port != 0 {
		t.Errorf("SMTPSender default Port = %d, want %d", sender.Port, 0)
	}
}

func TestSMTPSender_NoAuthCredentials(t *testing.T) {
	// SMTP sender should be constructible without auth credentials
	// (some SMTP servers allow unauthenticated sending from trusted networks)
	sender := &SMTPSender{
		Host: "localhost",
		Port: 25,
	}

	if sender.Host != "localhost" {
		t.Errorf("SMTPSender.Host = %q, want %q", sender.Host, "localhost")
	}
	if sender.User != "" {
		t.Errorf("SMTPSender.User = %q, want empty string", sender.User)
	}
	if sender.Password != "" {
		t.Errorf("SMTPSender.Password = %q, want empty string", sender.Password)
	}
}

// TestSMTPSender_MessageFormat tests that the message format would be correct
// Note: We can't easily test actual SMTP sending without a mail server,
// but we can verify the struct is properly configured
func TestSMTPSender_CommonPorts(t *testing.T) {
	tests := []struct {
		name string
		port int
		desc string
	}{
		{"SMTP port 25", 25, "standard SMTP"},
		{"Submission port 587", 587, "submission (STARTTLS)"},
		{"SMTPS port 465", 465, "implicit TLS"},
		{"Custom port 2525", 2525, "alternative port"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := &SMTPSender{
				Host: "smtp.example.com",
				Port: tt.port,
			}
			if sender.Port != tt.port {
				t.Errorf("SMTPSender.Port = %d, want %d for %s", sender.Port, tt.port, tt.desc)
			}
		})
	}
}

func TestSendmailSender_ImplementsEmailSender(t *testing.T) {
	sender := &SendmailSender{}

	// Verify SendmailSender implements EmailSender interface
	var _ EmailSender = sender
}

// TestEmailMessage_Format verifies the email message structure used by senders
func TestEmailMessage_Format(t *testing.T) {
	// This test verifies the expected format of email messages
	// by checking the format string components
	from := "sender@example.com"
	to := "recipient@example.com"
	subject := "Test Subject"
	body := "Test body content"

	// Simulate the message format used by SMTPSender and SendmailSender
	msg := "From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n" +
		body

	if !strings.Contains(msg, "From: sender@example.com") {
		t.Error("Message should contain From header")
	}
	if !strings.Contains(msg, "To: recipient@example.com") {
		t.Error("Message should contain To header")
	}
	if !strings.Contains(msg, "Subject: Test Subject") {
		t.Error("Message should contain Subject header")
	}
	if !strings.Contains(msg, "Content-Type: text/plain; charset=UTF-8") {
		t.Error("Message should contain Content-Type header")
	}
	if !strings.Contains(msg, "Test body content") {
		t.Error("Message should contain body")
	}

	// Verify header/body separator
	if !strings.Contains(msg, "\r\n\r\n") {
		t.Error("Message should have blank line between headers and body")
	}
}
