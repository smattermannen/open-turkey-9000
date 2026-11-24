package cloud

// mail.go — email to partners (SPEC §4.15). Devices never send email:
// they queue messages in the cloud (partner_send, partner_invite,
// unlock_request), and one sender (cmd/kc9k-mailer, run where the owner
// chooses, with the project's service role key) delivers the queue through
// a Mailer. The provider is the owner's choice (ROADMAP 2.13): SMTPMailer
// takes any SMTP service.

import (
	"fmt"
	"net/smtp"
	"strings"
)

// Mailer sends one email.
type Mailer interface {
	Send(to, subject, body string) error
}

// Outgoing is one queued email.
type Outgoing struct {
	ID      int64  `json:"id"`
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// SMTPMailer sends through an SMTP server with PLAIN authentication over
// TLS (STARTTLS, as net/smtp does on port 587).
type SMTPMailer struct {
	Host, Port, User, Password, From string
}

// Send sends one plain-text email.
func (m SMTPMailer) Send(to, subject, body string) error {
	if strings.ContainsAny(to+subject, "\r\n") {
		return fmt.Errorf("a header holds a line break")
	}
	msg := "From: " + m.From + "\r\nTo: " + to + "\r\nSubject: " + subject +
		"\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n" +
		strings.ReplaceAll(body, "\n", "\r\n")
	auth := smtp.PlainAuth("", m.User, m.Password, m.Host)
	return smtp.SendMail(m.Host+":"+m.Port, auth, m.From, []string{to}, []byte(msg))
}

// DeliverOutbox sends up to limit queued emails through m and marks each
// one sent; c must carry the service role key (the queue is closed to
// accounts). It returns how many were sent; one that fails stays queued
// and the first error is returned after the rest were tried.
func DeliverOutbox(c *Client, m Mailer, limit int) (int, error) {
	var out []Outgoing
	if err := c.rpc(nil, "outbox_take", map[string]any{"p_limit": limit}, &out); err != nil {
		return 0, err
	}
	var sent []int64
	var first error
	for _, o := range out {
		if err := m.Send(o.To, o.Subject, o.Body); err != nil {
			if first == nil {
				first = fmt.Errorf("email to %s: %w", o.To, err)
			}
			continue
		}
		sent = append(sent, o.ID)
	}
	if len(sent) > 0 {
		if err := c.rpc(nil, "outbox_done", map[string]any{"p_ids": sent}, nil); err != nil {
			return 0, err
		}
	}
	return len(sent), first
}
