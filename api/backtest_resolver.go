package api

import (
	"alpha/internal"
	"alpha/internal/app"
	"alpha/internal/domain"
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type backtestRequest struct {
	FactorOptions struct {
		Expression string  `json:"expression"`
		Intensity  float64 `json:"intensity"`
		Name       string  `json:"name"`
	} `json:"factorOptions"`
	BacktestStart        string `json:"backtestStart"`
	BacktestEnd          string `json:"backtestEnd"`
	SamplingIntervalUnit string `json:"samplingIntervalUnit"`

	AssetSelectionMode string  `json:"assetSelectionMode"`
	StartCash          float64 `json:"startCash"`

	AnchorPortfolioQuantities map[string]float64 `json:"anchorPortfolio"`
	NumSymbols                *int               `json:"numSymbols"`
}

type Trade struct {
	Action   string  `json:"action"`
	Quantity float64 `json:"quantity"`
	Symbol   string  `json:"symbol"`
	Price    float64 `json:"price"`
}

type backtestSnapshot struct {
	ValuePercentChange float64            `json:"valuePercentChange"`
	Value              float64            `json:"value"`
	Date               string             `json:"date"`
	AssetWeights       map[string]float64 `json:"assetWeights"`
	Trades             []Trade            `json:"trades"`
}

type backtestResponse struct {
	FactorName string                      `json:"factorName"`
	Snapshots  map[string]backtestSnapshot `json:"backtestSnapshots"`
}

func (h ApiHandler) backtest(c *gin.Context) {
	ctx := context.Background()
	tx, err := h.Db.BeginTx(
		ctx,
		&sql.TxOptions{
			Isolation: sql.LevelReadCommitted,
			ReadOnly:  true,
		},
	)
	if err != nil {
		returnErrorJson(err, c)
		return
	}
	var requestBody backtestRequest

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		returnErrorJson(err, c)
		return
	}

	backtestStartDate, err := time.Parse("2006-01-02", requestBody.BacktestStart)
	if err != nil {
		returnErrorJson(err, c)
		return
	}
	backtestEndDate, err := time.Parse("2006-01-02", requestBody.BacktestEnd)
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	samplingInterval := time.Hour * 24
	if strings.EqualFold(requestBody.SamplingIntervalUnit, "weekly") {
		samplingInterval *= 7
	} else if strings.EqualFold(requestBody.SamplingIntervalUnit, "monthly") {
		samplingInterval *= 30
	}

	assetSelectionMode, err := internal.NewAssetSelectionMode(requestBody.AssetSelectionMode)
	if err != nil {
		returnErrorJson(fmt.Errorf("could not parse asset selection mode: %w", err), c)
		return
	}

	startPortfolio := domain.Portfolio{
		Cash: requestBody.StartCash,
	}
	if *assetSelectionMode == internal.AssetSelectionMode_AnchorPortfolio {
		startPortfolio.Positions = domain.PositionsFromQuantity(requestBody.AnchorPortfolioQuantities)
	}

	backtestInput := app.BacktestInput{
		RoTx: tx,
		FactorOptions: app.FactorOptions{
			Expression: requestBody.FactorOptions.Expression,
			Intensity:  requestBody.FactorOptions.Intensity,
			Name:       requestBody.FactorOptions.Name,
		},
		BacktestStart:             backtestStartDate,
		BacktestEnd:               backtestEndDate,
		SamplingInterval:          samplingInterval,
		StartPortfolio:            startPortfolio,
		AnchorPortfolioQuantities: requestBody.AnchorPortfolioQuantities,
		AssetOptions: internal.AssetSelectionOptions{
			NumTickers:             requestBody.NumSymbols,
			AnchorPortfolioWeights: nil, // don't know this yet
			Mode:                   *assetSelectionMode,
		},
	}

	result, err := h.BacktestHandler.Backtest(backtestInput)
	if err != nil {
		returnErrorJson(fmt.Errorf("failed to run backtest: %w", err), c)
		return
	}

	snapshots := map[string]backtestSnapshot{
		result[0].Date.Format("2006-01-02"): {
			ValuePercentChange: 0,
			Value:              result[0].TotalValue,
			Date:               result[0].Date.Format("2006-01-02"),
			AssetWeights:       result[0].AssetWeights,
			Trades:             proposedTradesToApiTrades(result[0].ProposedTrades),
		},
	}

	for _, r := range result[1:] {
		snapshots[r.Date.Format("2006-01-02")] = backtestSnapshot{
			ValuePercentChange: 100 * (r.TotalValue - result[0].TotalValue) / result[0].TotalValue,
			Value:              r.TotalValue,
			Date:               r.Date.Format("2006-01-02"),
			AssetWeights:       r.AssetWeights,
			Trades:             proposedTradesToApiTrades(r.ProposedTrades),
		}
	}

	responseJson := backtestResponse{
		FactorName: backtestInput.FactorOptions.Name,
		Snapshots:  snapshots,
	}

	c.JSON(200, responseJson)
}

func proposedTradesToApiTrades(trades []domain.ProposedTrade) []Trade {
	out := make([]Trade, len(trades))
	for i, t := range trades {
		action := "BUY"
		if t.Quantity < 0 {
			action = "SELL"
		}
		out[i] = Trade{
			Action:   action,
			Quantity: math.Abs(t.Quantity),
			Symbol:   t.Symbol,
			Price:    t.ExpectedPrice,
		}
	}
	return out
}
