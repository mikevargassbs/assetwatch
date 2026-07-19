// Package mailer sends outbound email with an optional attachment via SMTP.
// It is deliberately minimal: no template engine, no queue. When SMTP isn't
// configured (no host set), Send returns ErrNotConfigured so callers can
// record the intended action without treating it as a hard failure — useful
// for environments where a mail relay hasn't been set up yet.
package mailer

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"mime/multipart"
	"net/smtp"
	"net/textproto"
)

var ErrNotConfigured = errors.New("smtp is not configured")

type Config struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

func (c Config) configured() bool {
	return c.Host != "" && c.From != ""
}

type Attachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

// Send delivers a plain-text email with an optional single attachment.
// Returns ErrNotConfigured if no SMTP host is set.
func Send(cfg Config, to, subject, body string, attachment *Attachment) error {
	return send(cfg, to, subject, "text/plain; charset=utf-8", body, attachment)
}

// SendHTML delivers an HTML email with an optional single attachment.
// Returns ErrNotConfigured if no SMTP host is set.
func SendHTML(cfg Config, to, subject, htmlBody string, attachment *Attachment) error {
	return send(cfg, to, subject, "text/html; charset=utf-8", htmlBody, attachment)
}

func send(cfg Config, to, subject, bodyContentType, body string, attachment *Attachment) error {
	if !cfg.configured() {
		return ErrNotConfigured
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	fmt.Fprintf(&buf, "From: %s\r\n", cfg.From)
	fmt.Fprintf(&buf, "To: %s\r\n", to)
	fmt.Fprintf(&buf, "Subject: %s\r\n", subject)
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&buf, "Content-Type: multipart/mixed; boundary=%s\r\n\r\n", writer.Boundary())

	bodyPart, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Type": {bodyContentType},
	})
	if err != nil {
		return fmt.Errorf("create body part: %w", err)
	}
	if _, err := bodyPart.Write([]byte(body)); err != nil {
		return fmt.Errorf("write body: %w", err)
	}

	if attachment != nil {
		attachPart, err := writer.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {attachment.ContentType},
			"Content-Transfer-Encoding": {"base64"},
			"Content-Disposition":       {fmt.Sprintf(`attachment; filename="%s"`, attachment.Filename)},
		})
		if err != nil {
			return fmt.Errorf("create attachment part: %w", err)
		}
		encoded := base64.StdEncoding.EncodeToString(attachment.Data)
		if _, err := attachPart.Write([]byte(encoded)); err != nil {
			return fmt.Errorf("write attachment: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}

	addr := cfg.Host + ":" + cfg.Port
	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}

	if err := smtp.SendMail(addr, auth, cfg.From, []string{to}, buf.Bytes()); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}
	return nil
}
