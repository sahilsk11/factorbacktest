package repository

import (
	"errors"
	"fmt"
	"strings"

	"github.com/resend/resend-go/v3"
)

// NewResendEmailRepository creates an EmailRepository backed by Resend.
// apiKey: Resend API key (re_xxx). fromEmail: must be on a domain that's
// been verified in the Resend dashboard. fromName is optional; when set,
// SendEmail formats the From header as "<fromName> <fromEmail>".
//
// Resend itself sits on top of Amazon SES for delivery, so deliverability
// is effectively the same backbone our SES repository targeted — we just
// get a friendlier API, a real free tier, and a CLI/MCP for ops.
func NewResendEmailRepository(apiKey, fromEmail, fromName string) (EmailRepository, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("resend: apiKey is required")
	}
	if strings.TrimSpace(fromEmail) == "" {
		return nil, errors.New("resend: fromEmail is required")
	}
	return &resendEmailRepositoryHandler{
		client:    resend.NewClient(apiKey),
		fromEmail: fromEmail,
		fromName:  fromName,
	}, nil
}

type resendEmailRepositoryHandler struct {
	client    *resend.Client
	fromEmail string
	fromName  string
}

func (h *resendEmailRepositoryHandler) SendEmail(to, subject, body string) error {
	from := h.fromEmail
	// RFC 5322 "Name <addr>" form so the inbox shows a friendly sender.
	// Skip when the caller already passed the formatted form, or when no
	// display name is configured.
	if h.fromName != "" && !strings.Contains(from, "<") {
		from = fmt.Sprintf("%s <%s>", h.fromName, h.fromEmail)
	}
	params := &resend.SendEmailRequest{
		From:    from,
		To:      []string{to},
		Subject: subject,
		Html:    body,
	}
	if _, err := h.client.Emails.Send(params); err != nil {
		return fmt.Errorf("resend send: %w", err)
	}
	return nil
}
