package api

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"factorbacktest/internal"
	"factorbacktest/internal/app"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"
)

type BacktestRequest struct {
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
	UserID                    *string            `json:"userID"`
}

type BacktestSnapshot struct {
	ValuePercentChange float64                          `json:"valuePercentChange"`
	Value              float64                          `json:"value"`
	Date               string                           `json:"date"`
	AssetMetrics       map[string]ScnapshotAssetMetrics `json:"assetMetrics"`
}

type ScnapshotAssetMetrics struct {
	AssetWeight                  float64  `json:"assetWeight"`
	FactorScore                  float64  `json:"factorScore"`
	PriceChangeTilNextResampling *float64 `json:"priceChangeTilNextResampling"`
}

type BacktestResponse struct {
	FactorName string                      `json:"factorName"`
	Snapshots  map[string]BacktestSnapshot `json:"backtestSnapshots"`
}

func (h ApiHandler) backtest(c *gin.Context) {
	performanceProfile := &internal.PerformanceProfile{}
	ctx := context.WithValue(context.Background(), "performanceProfile", performanceProfile)
	performanceProfile.Add("initialized")
	defer func() { performanceProfile.Print() }()

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
	defer tx.Rollback()

	var requestBody BacktestRequest

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
	} else if strings.EqualFold(requestBody.SamplingIntervalUnit, "yearly") {
		samplingInterval *= 365
	}

	assetSelectionMode, err := internal.NewAssetSelectionMode(requestBody.AssetSelectionMode)
	if err != nil {
		returnErrorJson(fmt.Errorf("could not parse asset selection mode: %w", err), c)
		return
	}

	// ensure the user input is valid
	err = saveUserStrategy(
		h.Db,
		h.UserStrategyRepository,
		requestBody,
	)
	if err != nil {
		returnErrorJson(err, c)
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

	performanceProfile.Add("starting backtest")

	result, err := h.BacktestHandler.Backtest(ctx, backtestInput)
	if err != nil {
		returnErrorJson(fmt.Errorf("failed to run backtest: %w", err), c)
		return
	}
	performanceProfile.Add("finished backtest")

	snapshots := map[string]BacktestSnapshot{}

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

		snapshots[r.Date.Format("2006-01-02")] = BacktestSnapshot{
			ValuePercentChange: pc,
			Value:              r.TotalValue,
			Date:               r.Date.Format("2006-01-02"),
			AssetMetrics:       joinAssetMetrics(r.AssetWeights, r.FactorScores, priceChangeTilNextResampling),
		}
	}

	responseJson := BacktestResponse{
		FactorName: backtestInput.FactorOptions.Name,
		Snapshots:  snapshots,
	}

	performanceProfile.Add("finished formatting")

	c.JSON(200, responseJson)
}

func joinAssetMetrics(
	weights map[string]float64,
	factorScores map[string]float64,
	priceChangeTilNextResampling map[string]float64,
) map[string]ScnapshotAssetMetrics {
	assetMetrics := map[string]*ScnapshotAssetMetrics{}
	for k, v := range weights {
		if _, ok := assetMetrics[k]; !ok {
			assetMetrics[k] = &ScnapshotAssetMetrics{}
		}
		assetMetrics[k].AssetWeight = v
	}
	for k, v := range factorScores {
		if _, ok := assetMetrics[k]; !ok {
			assetMetrics[k] = &ScnapshotAssetMetrics{}
		}
		assetMetrics[k].FactorScore = v
	}
	for k, v := range priceChangeTilNextResampling {
		if _, ok := assetMetrics[k]; !ok {
			assetMetrics[k] = &ScnapshotAssetMetrics{}
		}
		x := v // lol pointer math
		assetMetrics[k].PriceChangeTilNextResampling = &x
	}

	out := map[string]ScnapshotAssetMetrics{}
	for k := range assetMetrics {
		out[k] = *assetMetrics[k]
	}

	return out
}

func saveUserStrategy(
	db qrm.Executable,
	usr repository.UserStrategyRepository,
	requestBody BacktestRequest,
) error {
	type strategyInput struct {
		FactorName                string             `json:"factorName"`
		FactorExpression          string             `json:"factorExpression"`
		BacktestStart             string             `json:"backtestStart"`
		BacktestEnd               string             `json:"backtestEnd"`
		RebalanceInterval         string             `json:"rebalanceInterval"`
		AssetSelectionMode        string             `json:"assetSelectionMode"`
		StartCash                 float64            `json:"startCash"`
		AnchorPortfolioQuantities map[string]float64 `json:"anchorPortfolio"`
		NumSymbols                *int               `json:"numSymbols,omitempty"`
	}

	regex := regexp.MustCompile(`\s+`)
	cleanedExpression := regex.ReplaceAllString(requestBody.FactorOptions.Expression, "")

	// keep only selected fields bc we don't care about including
	// factor name and cash in hash
	si := &strategyInput{
		FactorExpression:          cleanedExpression,
		BacktestStart:             requestBody.BacktestStart,
		BacktestEnd:               requestBody.BacktestEnd,
		RebalanceInterval:         requestBody.SamplingIntervalUnit,
		AssetSelectionMode:        requestBody.AssetSelectionMode,
		AnchorPortfolioQuantities: requestBody.AnchorPortfolioQuantities,
		NumSymbols:                requestBody.NumSymbols,
	}
	siBytes, err := json.Marshal(si)
	if err != nil {
		return err
	}
	siHasher := sha256.New()
	siHasher.Write(siBytes)
	siHash := hex.EncodeToString(siHasher.Sum(nil))

	expressionHasher := sha256.New()
	expressionHasher.Write([]byte(cleanedExpression))
	expressionHash := hex.EncodeToString(expressionHasher.Sum(nil))

	// rewrite the saved JSON, including the ignored fields
	si.FactorName = requestBody.FactorOptions.Name
	si.StartCash = requestBody.StartCash
	siBytes, err = json.Marshal(si)
	if err != nil {
		return err
	}

	in := model.UserStrategy{
		StrategyInput:        string(siBytes),
		StrategyInputHash:    siHash,
		FactorExpressionHash: expressionHash,
	}

	if requestBody.UserID != nil {
		parsedUserID, err := uuid.Parse(*requestBody.UserID)
		if err == nil {
			in.UserID = &parsedUserID
		}
	}

	err = usr.Add(db, in)

	return err
}
