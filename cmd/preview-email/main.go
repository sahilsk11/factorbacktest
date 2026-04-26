// Command preview-email renders the strategy summary template with sample
// data and ships it through whichever EmailRepository cmd/util.go selected.
//
// Run with secrets-dev.json present:
//
//	ALPHA_ENV=dev go run ./cmd/preview-email -to=you@example.com
//
// Intended as a dev/QA tool — not registered in deploy paths. Safe to delete
// any time; nothing imports it.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	"factorbacktest/internal/service"
	"factorbacktest/internal/util"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func main() {
	to := flag.String("to", "", "recipient email")
	flag.Parse()
	if *to == "" {
		log.Fatal("--to=email is required")
	}

	secrets, err := util.LoadSecrets()
	if err != nil {
		log.Fatalf("load secrets: %v", err)
	}

	provider := os.Getenv("EMAIL_PROVIDER")
	if provider == "" {
		provider = "resend"
	}
	var repo repository.EmailRepository
	switch provider {
	case "ses":
		repo, err = repository.NewSESEmailRepository(secrets.SES.Region, secrets.SES.FromEmail)
	default:
		repo, err = repository.NewResendEmailRepository(secrets.Resend.APIKey, secrets.Resend.FromEmail, secrets.Resend.FromName)
	}
	if err != nil {
		log.Fatalf("build email repo: %v", err)
	}

	svc := service.NewEmailService(repo)

	user := &model.UserAccount{
		UserAccountID: uuid.New(),
		FirstName:     ptr("Sahil"),
		Email:         to, // *string from flag.String
	}

	now := time.Now()
	results := []domain.StrategySummaryResult{
		{
			StrategyID:          uuid.New(),
			StrategyName:        "Quality + Momentum 30",
			Date:                now,
			TotalPortfolioValue: decimal.NewFromInt(10000),
			Assets: []domain.StrategySummaryAsset{
				{Symbol: "MSFT", Weight: 0.18, FactorScore: 0.913, LastPrice: decimal.NewFromFloat(412.55)},
				{Symbol: "AAPL", Weight: 0.16, FactorScore: 0.881, LastPrice: decimal.NewFromFloat(231.40)},
				{Symbol: "NVDA", Weight: 0.14, FactorScore: 0.872, LastPrice: decimal.NewFromFloat(118.25)},
				{Symbol: "META", Weight: 0.13, FactorScore: 0.844, LastPrice: decimal.NewFromFloat(602.10)},
				{Symbol: "GOOGL", Weight: 0.12, FactorScore: 0.812, LastPrice: decimal.NewFromFloat(178.92)},
				{Symbol: "AVGO", Weight: 0.10, FactorScore: 0.795, LastPrice: decimal.NewFromFloat(212.41)},
				{Symbol: "ORCL", Weight: 0.09, FactorScore: 0.760, LastPrice: decimal.NewFromFloat(192.06)},
				{Symbol: "CRM", Weight: 0.08, FactorScore: 0.747, LastPrice: decimal.NewFromFloat(335.78)},
			},
		},
		{
			StrategyID:          uuid.New(),
			StrategyName:        "Low Vol Dividend",
			Date:                now,
			TotalPortfolioValue: decimal.NewFromInt(10000),
			Assets: []domain.StrategySummaryAsset{
				{Symbol: "JNJ", Weight: 0.30, FactorScore: 0.741, LastPrice: decimal.NewFromFloat(158.75)},
				{Symbol: "PG", Weight: 0.28, FactorScore: 0.726, LastPrice: decimal.NewFromFloat(173.92)},
				{Symbol: "KO", Weight: 0.22, FactorScore: 0.694, LastPrice: decimal.NewFromFloat(67.41)},
				{Symbol: "PEP", Weight: 0.20, FactorScore: 0.682, LastPrice: decimal.NewFromFloat(143.18)},
			},
		},
	}

	if err := svc.SendStrategySummaryEmail(user, results); err != nil {
		log.Fatalf("send: %v", err)
	}
	fmt.Printf("ok — sent strategy summary to %s via %s\n", *to, provider)
}

func ptr(s string) *string { return &s }
