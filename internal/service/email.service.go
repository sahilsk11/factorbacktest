package service

import (
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
)

// EmailService is responsible for the business logic around emails.
// It handles template rendering and email formatting, but does NOT
// compute strategy results - those are passed in as domain objects.
type EmailService interface {
	// SendStrategySummaryEmail sends an email to a user with their
	// strategy summary results. The strategyResults are pre-computed
	// domain objects containing what each strategy would buy.
	SendStrategySummaryEmail(
		user *model.UserAccount,
		strategyResults []domain.StrategySummaryResult,
	) error

	// GenerateStrategySummaryEmail generates the email content for a user's
	// strategy results. Returns the subject and HTML body.
	// This is used internally by SendStrategySummaryEmail but can also
	// be called separately for testing/preview purposes.
	GenerateStrategySummaryEmail(
		user *model.UserAccount,
		strategyResults []domain.StrategySummaryResult,
	) (string, string, error)
}

type emailServiceHandler struct {
	EmailRepository repository.EmailRepository
}

func NewEmailService(
	emailRepository repository.EmailRepository,
) EmailService {
	return &emailServiceHandler{
		EmailRepository: emailRepository,
	}
}

func (h *emailServiceHandler) SendStrategySummaryEmail(
	user *model.UserAccount,
	strategyResults []domain.StrategySummaryResult,
) error {
	// TODO: Implement:
	// 1. Call GenerateStrategySummaryEmail to get subject and body
	// 2. Use EmailRepository to send the email
	// 3. Handle errors appropriately
	return nil
}

func (h *emailServiceHandler) GenerateStrategySummaryEmail(
	user *model.UserAccount,
	strategyResults []domain.StrategySummaryResult,
) (string, string, error) {
	// TODO: Implement template generation:
	// 1. Format strategyResults into HTML table(s)
	// 2. Generate subject line (e.g., "Your Strategy Updates for [Date]")
	// 3. Return subject and HTML body
	return "", "", nil
}
