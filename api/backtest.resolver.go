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

type BacktestResponse struct {
	FactorName string                          `json:"factorName"`
	Snapshots  map[string]app.BacktestSnapshot `json:"backtestSnapshots"` // todo - figure this out
}

func (h ApiHandler) backtest(c *gin.Context) {
	performanceProfile := domain.NewPeformanceProfile()
	ctx := context.WithValue(context.Background(), "performanceProfile", performanceProfile)
	performanceProfile.Add("initialized")
	defer func() {
		performanceProfile.End()
		performanceProfile.Print()
	}()

	tx, err := h.Db.BeginTx(
		ctx,
		&sql.TxOptions{
			Isolation: sql.LevelReadCommitted,
			ReadOnly:  true,
		},
	)
	if err != nil {
		returnErrorJson(fmt.Errorf("failed to create transaction: %w", err), c)
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

	if backtestEndDate.Before(backtestStartDate) {
		returnErrorJson(fmt.Errorf("end date cannot be before start date"), c)
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

	var requestId *uuid.UUID
	requestIDAny, ok := c.Get("requestID")
	if ok {
		requestIDStr, ok := requestIDAny.(string)
		if ok {
			id, err := uuid.Parse(requestIDStr)
			if err == nil {
				requestId = &id
			}
		} else {
			fmt.Println("failed to convert to str")
		}
	} else {
		fmt.Println("missing from ctx")
	}
	// ensure the user input is valid
	err = saveUserStrategy(
		h.Db,
		h.UserStrategyRepository,
		requestBody,
		requestId,
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

	responseJson := BacktestResponse{
		FactorName: backtestInput.FactorOptions.Name,
		Snapshots:  result.Snapshots,
	}

	performanceProfile.Add("finished formatting")
	performanceProfile.End()

	h.LatencencyTrackingRepository.Add(*performanceProfile, requestId)

	c.JSON(200, responseJson)
}

func saveUserStrategy(
	db qrm.Executable,
	usr repository.UserStrategyRepository,
	requestBody BacktestRequest,
	requestId *uuid.UUID,
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
		RequestID:            requestId,
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
