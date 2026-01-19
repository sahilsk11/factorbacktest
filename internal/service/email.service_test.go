package service

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func initializeEmailService() (EmailService, error) {
	secretsFile := "../../secrets-dev.json"
	f, err := os.ReadFile(secretsFile)
	if err != nil {
		return nil, err
	}

	type secrets struct {
		SES struct {
			Region    string `json:"region"`
			FromEmail string `json:"fromEmail"`
		} `json:"ses"`
	}

	s := secrets{}
	err = json.Unmarshal(f, &s)
	if err != nil {
		return nil, err
	}

	if s.SES.Region == "" || s.SES.FromEmail == "" {
		return nil, fmt.Errorf("SES config not found in secrets")
	}

	emailRepo, err := repository.NewEmailRepository(s.SES.Region, s.SES.FromEmail)
	if err != nil {
		return nil, err
	}

	return NewEmailService(emailRepo), nil
}

// Test_emailServiceHandler_GenerateStrategySummaryEmail_Preview renders a template
// with sample data and saves it to a file for preview
func Test_emailServiceHandler_GenerateStrategySummaryEmail_Preview(t *testing.T) {
	// Set to false to run this test
	if true {
		t.Skip("Skipping template preview - set condition to false to run")
	}

	emailService, err := initializeEmailService()
	require.NoError(t, err)

	// Create sample domain objects
	user := &model.UserAccount{
		UserAccountID: uuid.New(),
		FirstName:     stringPtr("Sahil"),
		Email:         stringPtr("sahilkapur.a@gmail.com"),
	}

	strategyResults := []domain.StrategySummaryResult{
		{
			StrategyID:          uuid.New(),
			StrategyName:        "Momentum Strategy",
			Date:                time.Now(),
			TotalPortfolioValue: decimal.NewFromInt(10000),
			Assets: []domain.StrategySummaryAsset{
				{
					Symbol:      "AAPL",
					Weight:      0.25,
					FactorScore: 0.8542,
					LastPrice:   decimal.NewFromFloat(175.50),
				},
				{
					Symbol:      "GOOGL",
					Weight:      0.20,
					FactorScore: 0.7821,
					LastPrice:   decimal.NewFromFloat(142.30),
				},
				{
					Symbol:      "MSFT",
					Weight:      0.30,
					FactorScore: 0.9123,
					LastPrice:   decimal.NewFromFloat(378.90),
				},
			},
		},
		{
			StrategyID:          uuid.New(),
			StrategyName:        "Value Strategy",
			Date:                time.Now(),
			TotalPortfolioValue: decimal.NewFromInt(10000),
			Assets: []domain.StrategySummaryAsset{
				{
					Symbol:      "BRK.B",
					Weight:      0.40,
					FactorScore: 0.6543,
					LastPrice:   decimal.NewFromFloat(365.20),
				},
				{
					Symbol:      "JNJ",
					Weight:      0.35,
					FactorScore: 0.7234,
					LastPrice:   decimal.NewFromFloat(158.75),
				},
			},
		},
	}

	// Generate the email content
	subject, htmlBody, err := emailService.GenerateStrategySummaryEmail(user, strategyResults)
	require.NoError(t, err)

	// Save to file for preview
	previewFile := "/tmp/email_preview.html"
	err = os.WriteFile(previewFile, []byte(htmlBody), 0644)
	require.NoError(t, err)

	t.Logf("✓ Template rendered successfully!")
	t.Logf("")
	t.Logf("Subject: %s", subject)
	t.Logf("Preview saved to: %s", previewFile)
	t.Logf("Open it in your browser with:")
	t.Logf("  open %s", previewFile)

	// Optionally send as test email
	sendTestEmail := false // Set to true to also send as test email
	if sendTestEmail {
		err = emailService.SendStrategySummaryEmail(user, strategyResults)
		if err != nil {
			t.Logf("Failed to send test email: %v", err)
		} else {
			t.Logf("✓ Test email sent to %s", *user.Email)
		}
	}
}

func stringPtr(s string) *string {
	return &s
}
