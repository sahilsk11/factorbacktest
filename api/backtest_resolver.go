package api

import (
	"alpha/internal"
	"alpha/internal/app"
	"alpha/internal/domain"
	"context"
	"database/sql"
	"fmt"
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

type backtestSnapshot struct {
	ValuePercentChange float64                         `json:"valuePercentChange"`
	Value              float64                         `json:"value"`
	Date               string                          `json:"date"`
	AssetMetrics       map[string]snapshotAssetMetrics `json:"assetMetrics"`
}

type snapshotAssetMetrics struct {
	AssetWeight                  float64  `json:"assetWeight"`
	FactorScore                  float64  `json:"factorScore"`
	PriceChangeTilNextResampling *float64 `json:"priceChangeTilNextResampling"`
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

	snapshots := map[string]backtestSnapshot{}

	for i, r := range result {
		pc := 0.0
		if i != 0 {
			pc = 100 * (r.TotalValue - result[0].TotalValue) / result[0].TotalValue
		}
		priceChangeTilNextResampling := map[string]float64{}

		if i < len(result)-1 {
			nextResamplingDate := result[i+1].Date
			for symbol := range r.AssetWeights {
				startPrice, err := h.BacktestHandler.PriceRepository.Get(tx, symbol, r.Date)
				if err != nil {
					returnErrorJson(fmt.Errorf("failed to get price: %w", err), c)
					return
				}
				endPrice, err := h.BacktestHandler.PriceRepository.Get(tx, symbol, nextResamplingDate)
				if err != nil {
					returnErrorJson(fmt.Errorf("failed to get price: %w", err), c)
					return
				}
				priceChangeTilNextResampling[symbol] = 100 * (endPrice - startPrice) / startPrice
			}
		}

		snapshots[r.Date.Format("2006-01-02")] = backtestSnapshot{
			ValuePercentChange: pc,
			Value:              r.TotalValue,
			Date:               r.Date.Format("2006-01-02"),
			AssetMetrics:       joinAssetMetrics(r.AssetWeights, r.FactorScores, priceChangeTilNextResampling),
		}
	}

	responseJson := backtestResponse{
		FactorName: backtestInput.FactorOptions.Name,
		Snapshots:  snapshots,
	}

	c.JSON(200, responseJson)
}

func joinAssetMetrics(
	weights map[string]float64,
	factorScores map[string]float64,
	priceChangeTilNextResampling map[string]float64,
) map[string]snapshotAssetMetrics {
	assetMetrics := map[string]*snapshotAssetMetrics{}
	for k, v := range weights {
		if _, ok := assetMetrics[k]; !ok {
			assetMetrics[k] = &snapshotAssetMetrics{}
		}
		assetMetrics[k].AssetWeight = v
	}
	for k, v := range factorScores {
		if _, ok := assetMetrics[k]; !ok {
			assetMetrics[k] = &snapshotAssetMetrics{}
		}
		assetMetrics[k].FactorScore = v
	}
	for k, v := range priceChangeTilNextResampling {
		if _, ok := assetMetrics[k]; !ok {
			assetMetrics[k] = &snapshotAssetMetrics{}
		}
		x := v // lol pointer math
		assetMetrics[k].PriceChangeTilNextResampling = &x
	}

	out := map[string]snapshotAssetMetrics{}
	for k := range assetMetrics {
		out[k] = *assetMetrics[k]
	}

	return out
}
