package email

import (
	"bytes"
	"context"
	"fmt"
	"net/smtp"
)

type SMTPSender struct {
	host     string
	port     int
	from     string
	password string
}

func NewSMTPSender(host string, port int, from string, password string) *SMTPSender {
	return &SMTPSender{
		host:     host,
		port:     port,
		from:     from,
		password: password,
	}
}

func (s *SMTPSender) SendEmail(ctx context.Context, to string, subject string, htmlContent string) error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	auth := smtp.PlainAuth("", s.from, s.password, s.host)

	// Build the email headers and body
	var body bytes.Buffer
	body.WriteString(fmt.Sprintf("From: \"FitGlue\" <%s>\r\n", s.from))
	body.WriteString(fmt.Sprintf("To: %s\r\n", to))
	body.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	body.WriteString("MIME-Version: 1.0\r\n")
	body.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	body.WriteString("\r\n")
	body.WriteString(htmlContent)

	err := smtp.SendMail(addr, auth, s.from, []string{to}, body.Bytes())
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
