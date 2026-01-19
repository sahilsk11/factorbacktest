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

	// Get all users with email addresses
	users, err := h.UserAccountRepository.ListUsersWithEmail()
	if err != nil {
		return fmt.Errorf("failed to get users with email: %w", err)
	}

	lg.Infof("found %d users with email addresses", len(users))

	// Get latest trading day (assumes prices are already updated for today)
	latestTradingDay, err := h.PriceRepository.LatestTradingDay()
	if err != nil {
		return fmt.Errorf("failed to get latest trading day: %w", err)
	}

	// Use a reference portfolio value for calculations (e.g., $10,000)
	referencePortfolioValue := decimal.NewFromInt(10000)

	emailsSent := 0
	emailsFailed := 0

	// Process each user
	for _, user := range users {
		userLg := lg.With("userAccountID", user.UserAccountID.String())
		userCtx := context.WithValue(ctx, logger.ContextKey, userLg)

		// Get saved strategies for this user
		savedStrategies, err := h.StrategyRepository.List(repository.StrategyListFilter{
			SavedByUser: &user.UserAccountID,
		})
		if err != nil {
			lg.Warnf("failed to get saved strategies for user %s: %v", user.UserAccountID.String(), err)
			emailsFailed++
			continue
		}

		if len(savedStrategies) == 0 {
			lg.Debugf("user %s has no saved strategies, skipping", user.UserAccountID.String())
			continue
		}

		// Compute strategy summaries for each saved strategy
		strategyResults := []domain.StrategySummaryResult{}
		for _, strategy := range savedStrategies {
			summary, err := h.computeStrategySummary(userCtx, strategy, *latestTradingDay, referencePortfolioValue)
			if err != nil {
				lg.Warnf("failed to compute strategy summary for strategy %s (user %s): %v",
					strategy.StrategyID.String(), user.UserAccountID.String(), err)
				continue
			}
			strategyResults = append(strategyResults, *summary)
		}

		if len(strategyResults) == 0 {
			lg.Debugf("no valid strategy results for user %s, skipping email", user.UserAccountID.String())
			emailsFailed++
			continue
		}

		// Send email with strategy summaries
		err = h.EmailService.SendStrategySummaryEmail(&user, strategyResults)
		if err != nil {
			lg.Warnf("failed to send email to user %s: %v", user.UserAccountID.String(), err)
			emailsFailed++
			continue
		}

		lg.Infof("sent strategy summary email to user %s with %d strategies", user.UserAccountID.String(), len(strategyResults))
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
	for symbol, position := range computeTargetPortfolioResponse.TargetPortfolio.Positions {
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
			Quantity:    position.ExactQuantity,
			Weight:      weight,
			FactorScore: factorScore,
			Price:       price,
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
