package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"factorbacktest/internal/app"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	"factorbacktest/internal/util"
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
		Expression string `json:"expression"`
		Name       string `json:"name"`
	} `json:"factorOptions"`
	BacktestStart        string `json:"backtestStart"`
	BacktestEnd          string `json:"backtestEnd"`
	SamplingIntervalUnit string `json:"samplingIntervalUnit"`

	StartCash     float64 `json:"startCash"`
	AssetUniverse string  `json:"assetUniverse"`

	NumSymbols int     `json:"numSymbols"`
	UserID     *string `json:"userID"`
}

type BacktestResponse struct {
	FactorName     string                          `json:"factorName"`
	Snapshots      map[string]app.BacktestSnapshot `json:"backtestSnapshots"` // todo - figure this out
	LatestHoldings LatestHoldings                  `json:"latestHoldings"`
}

type LatestHoldings struct {
	Date   time.Time                           `json:"date"`
	Assets map[string]app.SnapshotAssetMetrics `json:"assets"`
}

func (h ApiHandler) backtest(c *gin.Context) {
	profile, endProfile := domain.NewProfile()
	ctx := context.WithValue(context.Background(), domain.ContextProfileKey, profile)

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

	assetUniverse := "SPY_TOP_80"
	if requestBody.AssetUniverse != "" {
		assetUniverse = requestBody.AssetUniverse
	}

	samplingInterval := time.Hour * 24
	if strings.EqualFold(requestBody.SamplingIntervalUnit, "weekly") {
		samplingInterval *= 7
	} else if strings.EqualFold(requestBody.SamplingIntervalUnit, "monthly") {
		samplingInterval *= 30
	} else if strings.EqualFold(requestBody.SamplingIntervalUnit, "yearly") {
		samplingInterval *= 365
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

	err = h.addNewSavedStrategy(
		c,
		requestBody.FactorOptions.Name,
		requestBody.FactorOptions.Expression,
		requestBody.SamplingIntervalUnit,
		assetUniverse,
		requestBody.NumSymbols,
		backtestStartDate,
		backtestEndDate,
	)
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	backtestInput := app.BacktestInput{
		FactorExpression:  requestBody.FactorOptions.Expression,
		FactorName:        requestBody.FactorOptions.Name,
		BacktestStart:     backtestStartDate,
		BacktestEnd:       backtestEndDate,
		RebalanceInterval: samplingInterval,
		StartingCash:      requestBody.StartCash,
		NumTickers:        requestBody.NumSymbols,
		AssetUniverse:     assetUniverse,
	}

	backtestSpan, endSpan := profile.StartNewSpan("running backtest")

	result, err := h.BacktestHandler.Backtest(domain.NewCtxWithSubProfile(ctx, backtestSpan), backtestInput)
	if err != nil {
		returnErrorJson(fmt.Errorf("failed to run backtest: %w", err), c)
		return
	}
	endSpan()

	responseJson := BacktestResponse{
		FactorName: backtestInput.FactorName,
		Snapshots:  result.Snapshots,
		LatestHoldings: LatestHoldings{
			Date:   result.LatestHoldings.Date,
			Assets: result.LatestHoldings.Assets,
		},
	}

	endProfile()
	err = h.LatencencyTrackingRepository.Add(*profile, requestId)
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	c.JSON(200, responseJson)
}

func saveUserStrategy(
	db qrm.Executable,
	usr repository.UserStrategyRepository,
	requestBody BacktestRequest,
	requestId *uuid.UUID,
) error {
	type strategyInput struct {
		FactorName        string  `json:"factorName"`
		FactorExpression  string  `json:"factorExpression"`
		BacktestStart     string  `json:"backtestStart"`
		BacktestEnd       string  `json:"backtestEnd"`
		RebalanceInterval string  `json:"rebalanceInterval"`
		StartCash         float64 `json:"startCash"`
		NumSymbols        int     `json:"numSymbols,omitempty"`
	}

	regex := regexp.MustCompile(`\s+`)
	cleanedExpression := regex.ReplaceAllString(requestBody.FactorOptions.Expression, "")

	// keep only selected fields bc we don't care about including
	// factor name and cash in hash
	si := &strategyInput{
		FactorExpression:  cleanedExpression,
		BacktestStart:     requestBody.BacktestStart,
		BacktestEnd:       requestBody.BacktestEnd,
		RebalanceInterval: requestBody.SamplingIntervalUnit,
		NumSymbols:        requestBody.NumSymbols,
	}
	siBytes, err := json.Marshal(si)
	if err != nil {
		return err
	}
	siHasher := sha256.New()
	siHasher.Write(siBytes)
	siHash := hex.EncodeToString(siHasher.Sum(nil))

	expressionHash := util.HashFactorExpression(requestBody.FactorOptions.Expression)

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
	if err != nil {
		return err
	}

	return err
}

func (m ApiHandler) addNewSavedStrategy(
	c *gin.Context,
	name string,
	expression string,
	rebalanceInterval string,
	assetUniverse string,
	numAssets int,
	start, end time.Time,
) error {
	ginUserAccountID, ok := c.Get("userAccountID")
	if !ok {
		return nil
	}
	userAccountIDStr, ok := ginUserAccountID.(string)
	if !ok {
		return fmt.Errorf("misformatted user account id")
	}
	if userAccountIDStr == "" {
		return nil
	}
	userAccountID, err := uuid.Parse(userAccountIDStr)
	if err != nil {
		return err
	}

	newModel := model.SavedStrategy{
		StrategyName:      name,
		FactorExpression:  expression,
		BacktestStart:     start,
		BacktestEnd:       end,
		RebalanceInterval: rebalanceInterval,
		NumAssets:         int32(numAssets),
		AssetUniverse:     assetUniverse,
		Bookmarked:        false,
		UserAccountID:     userAccountID,
	}
	_, err = m.SavedStrategyRepository.Add(newModel)
	if err != nil {
		return err
	}

	return nil
}
