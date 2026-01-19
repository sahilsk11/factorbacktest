package app

import (
	"context"
	"factorbacktest/internal/calculator"
	"factorbacktest/internal/data"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/logger"
	"factorbacktest/internal/repository"
	"factorbacktest/internal/service"
	"fmt"
	"time"

	"github.com/google/uuid"
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
	EmailPreferenceRepo     repository.EmailPreferenceRepository
	StrategyRepository      repository.StrategyRepository
	AssetUniverseRepository repository.AssetUniverseRepository
	PriceService            data.PriceService
	FactorExpressionService calculator.FactorExpressionService
	TickerRepository        repository.TickerRepository
	PriceRepository         repository.AdjustedPriceRepository
}

func NewStrategySummaryApp(
	emailService service.EmailService,
	userAccountRepository repository.UserAccountRepository,
	emailPreferenceRepo repository.EmailPreferenceRepository,
	strategyRepository repository.StrategyRepository,
	assetUniverseRepository repository.AssetUniverseRepository,
	priceService data.PriceService,
	factorExpressionService calculator.FactorExpressionService,
	tickerRepository repository.TickerRepository,
	priceRepository repository.AdjustedPriceRepository,
) StrategySummaryApp {
	return &strategySummaryAppHandler{
		EmailService:            emailService,
		UserAccountRepository:   userAccountRepository,
		EmailPreferenceRepo:     emailPreferenceRepo,
		StrategyRepository:      strategyRepository,
		AssetUniverseRepository: assetUniverseRepository,
		PriceService:            priceService,
		FactorExpressionService: factorExpressionService,
		TickerRepository:        tickerRepository,
		PriceRepository:         priceRepository,
	}
}

func (h *strategySummaryAppHandler) SendDailyStrategySummaries(ctx context.Context) error {
	lg := logger.FromContext(ctx)
	lg.Info("starting daily strategy summaries")

	// Determine which users are opted-in for this email type.
	optedInPrefs, err := h.EmailPreferenceRepo.ListOptedInByEmailType(model.EmailType_SavedStrategySummary)
	if err != nil {
		return fmt.Errorf("failed to list opted-in users: %w", err)
	}
	if len(optedInPrefs) == 0 {
		lg.Info("no opted-in users found; skipping")
		return nil
	}

	// Get latest trading day (assumes prices are already updated for today)
	latestTradingDay, err := h.PriceRepository.LatestTradingDay()
	if err != nil {
		return fmt.Errorf("failed to get latest trading day: %w", err)
	}

	emailsSent := 0
	emailsFailed := 0

	// Process each user
	for _, optInPreference := range optedInPrefs {
		err := h.processSavedStrategyEmail(ctx, optInPreference, *latestTradingDay)
		if err != nil {
			emailsFailed++
			continue
		}
		emailsSent++
	}

	lg.Infof("daily strategy summaries completed: %d emails sent, %d failed", emailsSent, emailsFailed)
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
	// 1. Get assets in the strategy's universe
	universe, err := h.AssetUniverseRepository.GetAssets(strategy.AssetUniverse)
	if err != nil {
		return nil, fmt.Errorf("failed to get assets in universe %s: %w", strategy.AssetUniverse, err)
	}

	if len(universe) == 0 {
		return nil, fmt.Errorf("universe %s has no assets", strategy.AssetUniverse)
	}

	// Extract symbols and build ticker ID map
	universeSymbols := []string{}
	tickerIDMap := map[string]uuid.UUID{}
	for _, ticker := range universe {
		universeSymbols = append(universeSymbols, ticker.Symbol)
		tickerIDMap[ticker.Symbol] = ticker.TickerID
	}

	// 2. Get prices for the date
	priceMap, err := h.PriceRepository.GetManyOnDay(universeSymbols, date)
	if err != nil {
		return nil, fmt.Errorf("failed to get prices for date %s: %w", date.Format(time.DateOnly), err)
	}

	// 3. Calculate factor scores for the date
	// Use CalculateFactorScores for a single date
	factorScoresByDay, err := h.FactorExpressionService.CalculateFactorScores(ctx, []time.Time{date}, universe, strategy.FactorExpression)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate factor scores: %w", err)
	}

	scoresOnDay, ok := factorScoresByDay[date]
	if !ok {
		return nil, fmt.Errorf("factor scores missing for date %s", date.Format(time.DateOnly))
	}

	// Convert factor scores to map[string]*float64 format expected by ComputeTargetPortfolio
	// SymbolScores is already map[string]*float64, so we can use it directly
	factorScoresMap := scoresOnDay.SymbolScores

	// 4. Compute target portfolio using calculator.ComputeTargetPortfolio
	computeTargetPortfolioResponse, err := calculator.ComputeTargetPortfolio(calculator.ComputeTargetPortfolioInput{
		Date:             date,
		TargetNumTickers: int(strategy.NumAssets),
		FactorScores:     factorScoresMap,
		PortfolioValue:   referencePortfolioValue,
		PriceMap:         priceMap,
		TickerIDMap:      tickerIDMap,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to compute target portfolio: %w", err)
	}

	// 5. Convert to StrategySummaryResult domain object
	assets := []domain.StrategySummaryAsset{}
	for symbol := range computeTargetPortfolioResponse.TargetPortfolio.Positions {
		weight, ok := computeTargetPortfolioResponse.AssetWeights[symbol]
		if !ok {
			continue
		}
		factorScore, ok := computeTargetPortfolioResponse.FactorScores[symbol]
		if !ok {
			continue
		}
		price, ok := priceMap[symbol]
		if !ok {
			continue
		}

		assets = append(assets, domain.StrategySummaryAsset{
			Symbol:      symbol,
			Weight:      weight,
			FactorScore: factorScore,
			LastPrice:   price,
		})
	}

	return &domain.StrategySummaryResult{
		StrategyID:          strategy.StrategyID,
		StrategyName:        strategy.StrategyName,
		Date:                date,
		Assets:              assets,
		TotalPortfolioValue: referencePortfolioValue,
	}, nil
}

type SavedStrategyEmailResult struct {
	Result *domain.StrategySummaryResult
	Error  error
}

func (h *strategySummaryAppHandler) processSavedStrategyEmail(
	ctx context.Context,
	optInPreference model.EmailPreference,
	date time.Time,
) error {
	userAccountID := optInPreference.UserAccountID
	userAccount, err := h.UserAccountRepository.GetByID(userAccountID)
	if err != nil {
		return fmt.Errorf("failed to get user account: %w", err)
	}
	if userAccount == nil {
		return fmt.Errorf("user account not found")
	}

	// Get saved strategies for this user
	savedStrategies, err := h.StrategyRepository.List(repository.StrategyListFilter{
		SavedByUser: &userAccountID,
	})
	if err != nil {
		return fmt.Errorf("failed to get saved strategies: %w", err)
	}

	if len(savedStrategies) == 0 {
		return nil
	}

	// arbitrary reference portfolio value. required for calculations.
	referencePortfolioValue := decimal.NewFromInt(10000)

	// Compute strategy summaries for each saved strategy
	strategyResults := []domain.StrategySummaryResult{}
	for _, strategy := range savedStrategies {
		summary, err := h.computeStrategySummary(ctx, strategy, date, referencePortfolioValue)
		if err != nil {
			strategyResults = append(strategyResults, domain.StrategySummaryResult{
				StrategyID:          strategy.StrategyID,
				StrategyName:        strategy.StrategyName,
				Date:                date,
				TotalPortfolioValue: referencePortfolioValue,
				Error:               err,
			})
			continue
		}
		strategyResults = append(strategyResults, *summary)
	}

	// Send email with strategy summaries
	err = h.EmailService.SendStrategySummaryEmail(userAccount, strategyResults)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
