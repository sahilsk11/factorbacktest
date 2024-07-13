package l3_service

import (
	"factorbacktest/internal/domain"

	"github.com/google/uuid"
)

// figure out all the target portfolios
// and compare them to current portfolio

type TradingService interface {
	Calculate(strategyInvestmentID uuid.UUID) ([]domain.ProposedTrade, error)
}

// func (h BacktestHandler) transitionToTarget(
// 	currentPortfolio domain.Portfolio,
// 	targetPortfolio domain.Portfolio,
// 	priceMap map[string]float64,
// ) ([]domain.ProposedTrade, error) {
// 	trades := []domain.ProposedTrade{}
// 	prevPositions := currentPortfolio.Positions
// 	targetPositions := targetPortfolio.Positions

// 	for symbol, position := range targetPositions {
// 		diff := position.Quantity
// 		prevPosition, ok := prevPositions[symbol]
// 		if ok {
// 			diff = position.Quantity - prevPosition.Quantity
// 		}
// 		if diff != 0 {
// 			trades = append(trades, domain.ProposedTrade{
// 				Symbol:        symbol,
// 				Quantity:      diff,
// 				ExpectedPrice: priceMap[symbol],
// 			})
// 		}
// 	}
// 	for symbol, position := range prevPositions {
// 		if _, ok := targetPositions[symbol]; !ok {
// 			trades = append(trades, domain.ProposedTrade{
// 				Symbol:        symbol,
// 				Quantity:      -position.Quantity,
// 				ExpectedPrice: priceMap[symbol],
// 			})
// 		}
// 	}

// 	return trades, nil
// }
