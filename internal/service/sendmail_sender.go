package service

import (
	"fmt"
	"os/exec"
	"strings"
)

// SendmailSender sends email via the local `sendmail` binary.
type SendmailSender struct{}

// SendRawEmail sends an email via the system sendmail command.
func (s *SendmailSender) SendRawEmail(from, to, subject, body string) error {
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from, to, subject, body)

	cmd := exec.Command("sendmail", "-t", "-oi")
	cmd.Stdin = strings.NewReader(msg)
	return cmd.Run()
}
