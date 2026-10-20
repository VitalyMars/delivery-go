package email

import (
	"context"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

type Sender struct{ host, port, from string }

func NewSender(host, port, from string) *Sender {
	return &Sender{strings.TrimSpace(host), strings.TrimSpace(port), strings.TrimSpace(from)}
}

// Send targets local Mailpit. Cancellation closes the socket, including during DATA.
func (s *Sender) Send(ctx context.Context, to, subject, body string) error {
	for _, address := range []string{s.from, to} {
		parsed, err := mail.ParseAddress(address)
		if err != nil || parsed.Address != address || strings.ContainsAny(address, "\r\n") {
			return fmt.Errorf("invalid email address")
		}
	}
	if subject == "" || strings.ContainsAny(subject, "\r\n") {
		return fmt.Errorf("invalid subject")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(s.host, s.port))
	if err != nil {
		return fmt.Errorf("dial SMTP: %w", err)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	if err = conn.SetDeadline(deadline); err != nil {
		return err
	}
	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return err
	}
	defer client.Close()
	if err = client.Mail(s.from); err != nil {
		return err
	}
	if err = client.Rcpt(to); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	message := "From: " + s.from + "\r\nTo: " + to + "\r\nSubject: " + mime.QEncoding.Encode("UTF-8", subject) + "\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + body + "\r\n"
	if _, err = writer.Write([]byte(message)); err != nil {
		return err
	}
	// The DATA reply is the acceptance boundary. A later QUIT failure is irrelevant.
	if err = writer.Close(); err != nil {
		return err
	}
	_ = client.Quit()
	return nil
}
