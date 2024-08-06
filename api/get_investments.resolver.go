package api

import (
	"factorbacktest/internal/repository"
	l3_service "factorbacktest/internal/service/l3"
	"fmt"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type GetInvestmentsResponse struct {
	InvestmentID          uuid.UUID     `json:"investmentID"`
	OriginalAmountDollars int32         `json:"originalAmountDollars"`
	StartDate             string        `json:"startDate"`
	Strategy              Strategy      `json:"strategy"`
	Holdings              []Holdings    `json:"holdings"`
	PercentReturnFraction float64       `json:"percentReturnFraction"`
	CurrentValue          float64       `json:"currentValue"`
	CompletedTrades       []FilledTrade `json:"completedTrades"`
}

type Holdings struct {
	Symbol      string  `json:"symbol"`
	Quantity    float64 `json:"quantity"`
	MarketValue float64 `json:"marketValue"`
}

type FilledTrade struct {
	Symbol    string  `json:"symbol"`
	Quantity  float64 `json:"quantity"`
	FillPrice float64 `json:"fillPrice"`
	FilledAt  string  `json:"filledAt"`
}

type Strategy struct {
	StrategyID        uuid.UUID `json:"strategyID"`
	StrategyName      string    `json:"strategyName"`
	FactorExpression  string    `json:"factorExpression"`
	NumAssets         int32     `json:"numAssets"`
	AssetUniverse     string    `json:"assetUniverse"`
	RebalanceInterval string    `json:"rebalanceInterval"`
}

func (m ApiHandler) getInvestments(c *gin.Context) {
	ginUserAccountID, ok := c.Get("userAccountID")
	if !ok {
		returnErrorJson(fmt.Errorf("must be logged in to view investments"), c)
		return
	}
	userAccountIDStr, ok := ginUserAccountID.(string)
	if !ok {
		returnErrorJson(fmt.Errorf("misformatted user account id"), c)
		return
	}

	userAccountID, err := uuid.Parse(userAccountIDStr)
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	investments, err := m.InvestmentRepository.List(repository.StrategyInvestmentListFilter{
		UserAccountIDs: []uuid.UUID{userAccountID},
	})
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	statsByInvestment := map[uuid.UUID]l3_service.GetStatsResponse{}
	for _, i := range investments {
		stats, err := m.InvestmentService.GetStats(i.InvestmentID)
		if err != nil {
			returnErrorJson(err, c)
			return
		}
		statsByInvestment[i.InvestmentID] = *stats
	}

	out := getInvestmentsResponseFromDomain(statsByInvestment)

	c.JSON(200, out)
}

func getInvestmentsResponseFromDomain(in map[uuid.UUID]l3_service.GetStatsResponse) []GetInvestmentsResponse {
	out := []GetInvestmentsResponse{}
	for investmentID, stats := range in {
		holdings := []Holdings{}
		for _, h := range stats.Holdings {
			holdings = append(holdings, Holdings{
				Symbol:      h.Symbol,
				Quantity:    h.ExactQuantity.InexactFloat64(),
				MarketValue: h.Value.InexactFloat64(),
			})
		}

		completedTrades := []FilledTrade{}
		for _, t := range stats.CompletedTrades {
			completedTrades = append(completedTrades, FilledTrade{
				Symbol:    t.Symbol,
				Quantity:  t.Quantity.InexactFloat64(),
				FillPrice: t.FillPrice.InexactFloat64(),
				FilledAt:  t.FilledAt.Format(time.RFC3339),
			})
		}

		out = append(out, GetInvestmentsResponse{
			InvestmentID:          investmentID,
			OriginalAmountDollars: stats.OriginalAmount,
			StartDate:             stats.StartDate.Format(time.DateOnly),
			Strategy: Strategy{
				StrategyID:        stats.Strategy.StrategyID,
				StrategyName:      stats.Strategy.StrategyName,
				FactorExpression:  stats.Strategy.FactorExpression,
				NumAssets:         stats.Strategy.NumAssets,
				AssetUniverse:     stats.Strategy.AssetUniverse,
				RebalanceInterval: stats.Strategy.RebalanceInterval,
			},
			Holdings:              holdings,
			PercentReturnFraction: stats.PercentReturnFraction.InexactFloat64(),
			CurrentValue:          stats.CurrentValue.InexactFloat64(),
			CompletedTrades:       completedTrades,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].StartDate < out[j].StartDate
	})

	return out
}
