package service

import (
	"context"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/repository"
	"time"
)

// EmailService is responsible for the business logic around emails.
// It determines what content goes in emails, handles template rendering,
// and orchestrates email sending for business use cases.
type EmailService interface {
	// SendDailyStrategySummaries sends daily strategy summary emails
	// to all users who have saved strategies and email addresses.
	// It computes what each strategy would buy today and sends a summary.
	SendDailyStrategySummaries(ctx context.Context) error

	// GenerateStrategySummaryEmail generates the email content for a user's
	// saved strategies. Returns the subject and HTML body.
	GenerateStrategySummaryEmail(user *model.UserAccount, strategies []model.Strategy, date time.Time) (string, string, error)
}

type emailServiceHandler struct {
	EmailRepository      repository.EmailRepository
	UserAccountRepository repository.UserAccountRepository
	StrategyRepository   repository.StrategyRepository
	// TODO: Add other dependencies needed for computing strategy results
	// PriceService, FactorExpressionService, etc.
}

func NewEmailService(
	emailRepository repository.EmailRepository,
	userAccountRepository repository.UserAccountRepository,
	strategyRepository repository.StrategyRepository,
) EmailService {
	return &emailServiceHandler{
		EmailRepository:      emailRepository,
		UserAccountRepository: userAccountRepository,
		StrategyRepository:   strategyRepository,
	}
}

func (h *emailServiceHandler) SendDailyStrategySummaries(ctx context.Context) error {
	// TODO: Implement business logic:
	// 1. Get all users with email addresses
	// 2. For each user, get their saved strategies
	// 3. For each strategy, compute what it would buy today
	// 4. Generate email content
	// 5. Send emails
	return nil
}

func (h *emailServiceHandler) GenerateStrategySummaryEmail(user *model.UserAccount, strategies []model.Strategy, date time.Time) (string, string, error) {
	// TODO: Implement template generation:
	// 1. For each strategy, compute target portfolio
	// 2. Format results into HTML table
	// 3. Generate subject line
	// 4. Return subject and HTML body
	return "", "", nil
}
