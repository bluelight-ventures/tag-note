package service

import (
	"fmt"
	"net/smtp"
)

// SMTPSender sends email via a standard SMTP relay.
type SMTPSender struct {
	Host     string
	Port     int
	User     string
	Password string
}

// SendRawEmail sends an email via SMTP.
func (s *SMTPSender) SendRawEmail(from, to, subject, body string) error {
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from, to, subject, body)

	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)

	var auth smtp.Auth
	if s.User != "" && s.Password != "" {
		auth = smtp.PlainAuth("", s.User, s.Password, s.Host)
	}

	return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
}
