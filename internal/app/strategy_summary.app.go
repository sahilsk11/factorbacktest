package app

import (
	"context"
	"factorbacktest/internal/calculator"
	"factorbacktest/internal/data"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	"factorbacktest/internal/service"
	"time"

	"github.com/shopspring/decimal"
)

// StrategySummaryApp orchestrates the business logic for sending daily
// strategy summary emails. It coordinates between multiple services to:
// 1. Get users with saved strategies
// 2. Compute what each strategy would buy today
// 3. Send emails via EmailService
type StrategySummaryApp interface {
	SendDailyStrategySummaries(ctx context.Context) error
}

type strategySummaryAppHandler struct {
	EmailService            service.EmailService
	UserAccountRepository   repository.UserAccountRepository
	StrategyRepository      repository.StrategyRepository
	AssetUniverseRepository repository.AssetUniverseRepository
	PriceService            data.PriceService
	FactorExpressionService calculator.FactorExpressionService
	TickerRepository        repository.TickerRepository
}

func NewStrategySummaryApp(
	emailService service.EmailService,
	userAccountRepository repository.UserAccountRepository,
	strategyRepository repository.StrategyRepository,
	assetUniverseRepository repository.AssetUniverseRepository,
	priceService data.PriceService,
	factorExpressionService calculator.FactorExpressionService,
	tickerRepository repository.TickerRepository,
) StrategySummaryApp {
	return &strategySummaryAppHandler{
		EmailService:            emailService,
		UserAccountRepository:   userAccountRepository,
		StrategyRepository:      strategyRepository,
		AssetUniverseRepository: assetUniverseRepository,
		PriceService:            priceService,
		FactorExpressionService: factorExpressionService,
		TickerRepository:        tickerRepository,
	}
}

func (h *strategySummaryAppHandler) SendDailyStrategySummaries(ctx context.Context) error {
	// TODO: Implement orchestration logic:
	// 1. Get all users with email addresses and saved strategies
	// 2. For each user:
	//    a. Get their saved strategies
	//    b. For each strategy, compute what it would buy today
	//       - Get latest prices
	//       - Calculate factor scores
	//       - Compute target portfolio
	//       - Convert to StrategySummaryResult domain object
	//    c. Call EmailService.SendStrategySummaryEmail with results
	// 3. Handle errors gracefully (log but continue processing)
	return nil
}

// computeStrategySummary computes what a strategy would buy on a given date
// and returns a domain object with the results
func (h *strategySummaryAppHandler) computeStrategySummary(
	ctx context.Context,
	strategy model.Strategy,
	date time.Time,
	referencePortfolioValue decimal.Decimal,
) (*domain.StrategySummaryResult, error) {
	// TODO: Implement:
	// 1. Get assets in the strategy's universe
	// 2. Get latest prices for those assets
	// 3. Calculate factor scores for the date
	// 4. Compute target portfolio using calculator.ComputeTargetPortfolio
	// 5. Convert to StrategySummaryResult domain object
	return nil, nil
}
