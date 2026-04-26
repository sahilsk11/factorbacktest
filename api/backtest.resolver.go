package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"factorbacktest/internal/calculator"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/logger"
	"factorbacktest/internal/progress"
	"factorbacktest/internal/repository"
	"factorbacktest/internal/service"
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
	FactorName       string                              `json:"factorName"`
	StrategyID       uuid.UUID                           `json:"strategyID"`
	Snapshots        map[string]service.BacktestSnapshot `json:"backtestSnapshots"` // todo - figure this out
	LatestHoldings   LatestHoldings                      `json:"latestHoldings"`
	SharpeRatio      *float64                            `json:"sharpeRatio"`
	AnnualizedReturn *float64                            `json:"annualizedReturn"`
	AnnualizedStdev  *float64                            `json:"annualizedStandardDeviation"`
}

type LatestHoldings struct {
	Date   time.Time                               `json:"date"`
	Assets map[string]service.SnapshotAssetMetrics `json:"assets"`
}

// backtest is the legacy synchronous endpoint. It returns the entire result
// in a single JSON response. We keep it untouched so existing FE code (and
// any external integrations) keep working while we cut the new SSE-based
// /backtest/stream over.
func (h ApiHandler) backtest(c *gin.Context) {
	requestBody, err := parseBacktestRequest(c)
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	// No reporter installed = progress.Step is a no-op, so the resolver +
	// service path is identical to before.
	resp, runErr := h.runBacktest(c, *requestBody)
	if runErr != nil {
		returnErrorJsonCode(runErr.Err, c, runErr.Code)
		return
	}

	c.JSON(200, resp)
}

// backtestStream runs the same computation as `backtest` but returns
// Server-Sent Events so the frontend can render a step-by-step loading UX.
//
// Wire format: each frame is a single SSE `data:` line containing a JSON
// progress.Event. The stream terminates with either a `result` event
// carrying the full BacktestResponse, or an `error` event. HTTP status is
// always 200 once the stream begins; transport-level errors must be
// surfaced in-band because we've already committed the response.
func (h ApiHandler) backtestStream(c *gin.Context) {
	requestBody, err := parseBacktestRequest(c)
	if err != nil {
		// Safe to return a normal error here — we haven't started the
		// SSE stream yet (no headers written).
		returnErrorJson(err, c)
		return
	}

	// Standard SSE response headers. `X-Accel-Buffering: no` disables
	// nginx-style proxy buffering; without it intermediaries may hold
	// onto frames until the response closes, defeating the entire point.
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(200)

	sseW, err := progress.NewSSEWriter(c.Writer)
	if err != nil {
		// gin's writer always supports flushing, so this is effectively
		// unreachable — but we don't want to silently swallow it if
		// something underneath ever changes.
		returnErrorJson(err, c)
		return
	}
	defer sseW.Close()

	reporter := progress.NewSSEReporter(sseW)
	c.Set(progressReporterCtxKey, reporter)

	resp, runErr := h.runBacktest(c, *requestBody)
	if runErr != nil {
		_ = sseW.Send(progress.Event{Type: "error", Error: runErr.Err.Error()})
		// We don't set a non-200 status here — the stream is already
		// open. The client distinguishes terminal states via event Type.
		logger.FromContext(c).Errorf("backtest stream failed: %s", runErr.Err.Error())
		return
	}

	if err := sseW.Send(progress.Event{Type: "result", Result: resp}); err != nil {
		logger.FromContext(c).Errorf("failed to send terminal result frame: %s", err.Error())
	}
}

// progressReporterCtxKey is the gin-context key under which we stash the
// per-request progress.Reporter. runBacktest reads it and threads the
// reporter onto the Go context that flows through the service layer.
const progressReporterCtxKey = "progressReporter"

// runBacktestErr lets us bubble up an error along with the HTTP status the
// synchronous endpoint should use. The streaming endpoint ignores the code
// and just emits an error event.
type runBacktestErr struct {
	Err  error
	Code int
}

// parseBacktestRequest pulls the BacktestRequest out of the gin context and
// validates the fields the resolver cares about. Splitting this out lets
// both endpoints share validation while keeping their response handling
// distinct (JSON vs SSE).
func parseBacktestRequest(c *gin.Context) (*BacktestRequest, error) {
	var requestBody BacktestRequest
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		return nil, err
	}

	backtestStartDate, err := time.Parse("2006-01-02", requestBody.BacktestStart)
	if err != nil {
		return nil, err
	}
	backtestEndDate, err := time.Parse("2006-01-02", requestBody.BacktestEnd)
	if err != nil {
		return nil, err
	}
	if backtestEndDate.Before(backtestStartDate) {
		return nil, fmt.Errorf("end date cannot be before start date")
	}

	return &requestBody, nil
}

// runBacktest is the shared core of the synchronous and streaming
// endpoints. It owns: persisting the strategy, invoking the backtest
// service, computing metrics, and assembling the BacktestResponse. The
// progress.Reporter (if any) is read off the gin context and threaded onto
// the Go context so the service layer can emit step events at its own
// phase boundaries.
func (h ApiHandler) runBacktest(c *gin.Context, requestBody BacktestRequest) (*BacktestResponse, *runBacktestErr) {
	log := logger.FromContext(c)
	profile, endProfile := domain.NewProfile()
	ctx := context.WithValue(context.Background(), domain.ContextProfileKey, profile)

	// Promote the Reporter (set by the SSE endpoint, absent for the
	// synchronous endpoint) onto the Go context so service-layer code can
	// emit progress events without depending on gin.
	if v, exists := c.Get(progressReporterCtxKey); exists {
		if r, ok := v.(progress.Reporter); ok && r != nil {
			ctx = progress.WithReporter(ctx, r)
		}
	}

	// Validation already done in parseBacktestRequest, but we still need
	// the parsed dates here.
	backtestStartDate, _ := time.Parse("2006-01-02", requestBody.BacktestStart)
	backtestEndDate, _ := time.Parse("2006-01-02", requestBody.BacktestEnd)

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
			log.Warn("failed to convert to str")
		}
	} else {
		log.Warn("request id missing from ctx")
	}

	// deprecated - uses the old user_strategy model
	if err := saveUserStrategy(h.Db, h.UserStrategyRepository, requestBody, requestId); err != nil {
		return nil, &runBacktestErr{Err: err, Code: 500}
	}

	// new version - save the strategy
	insertedStrategy, err := h.addNewStrategy(
		c,
		requestBody.FactorOptions.Name,
		requestBody.FactorOptions.Expression,
		requestBody.SamplingIntervalUnit,
		assetUniverse,
		requestBody.NumSymbols,
	)
	if err != nil {
		return nil, &runBacktestErr{Err: err, Code: 500}
	}

	backtestInput := service.BacktestInput{
		FactorExpression:  requestBody.FactorOptions.Expression,
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
		return nil, &runBacktestErr{Err: fmt.Errorf("failed to run backtest: %w", err), Code: 500}
	}
	endSpan()

	// calculate stats — this is fast enough that we group it with the
	// "save run" call under a single user-visible "metrics" step.
	endMetricsStep := progress.Step(ctx, "metrics", "Computing performance metrics")
	metrics, err := h.StrategyService.CalculateMetrics(ctx, insertedStrategy.StrategyID, result.Results)
	if err != nil {
		log.Errorf("failed to calculate metrics: %w", err)
		metrics = &calculator.CalculateMetricsResult{}
	}

	newRunModel := model.StrategyRun{
		StrategyID: insertedStrategy.StrategyID,
		StartDate:  backtestStartDate,
		EndDate:    backtestEndDate,
	}
	if metrics != nil {
		newRunModel.SharpeRatio = &metrics.SharpeRatio
		newRunModel.AnnualizedReturn = &metrics.AnnualizedReturn
		newRunModel.AnnualuzedStdev = &metrics.AnnualizedStdev
	}

	if _, err := h.StrategyRepository.AddRun(newRunModel); err != nil {
		log.Errorf("failed to add strategy run: %w", err)
	}
	endMetricsStep()

	responseJson := &BacktestResponse{
		StrategyID: insertedStrategy.StrategyID,
		FactorName: requestBody.FactorOptions.Name,
		Snapshots:  result.Snapshots,
		LatestHoldings: LatestHoldings{
			Date:   result.LatestHoldings.Date,
			Assets: result.LatestHoldings.Assets,
		},
		AnnualizedReturn: &metrics.AnnualizedReturn,
		SharpeRatio:      &metrics.SharpeRatio,
		AnnualizedStdev:  &metrics.AnnualizedStdev,
	}

	endProfile()

	// Latency tracking — best effort. Failing this should not fail the
	// backtest; the original synchronous handler treated this as fatal,
	// but that's overzealous: the user already got their result.
	if err := h.LatencencyTrackingRepository.Add(*profile, requestId); err != nil {
		log.Errorf("failed to record latency profile: %s", err.Error())
	}

	return responseJson, nil
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

func (m ApiHandler) addNewStrategy(
	c *gin.Context,
	name string,
	expression string,
	rebalanceInterval string,
	assetUniverse string,
	numAssets int,
) (*model.Strategy, error) {
	var userAccountID *uuid.UUID
	ginUserAccountID, ok := c.Get("userAccountID")
	if ok {
		userAccountIDStr, ok := ginUserAccountID.(string)
		if !ok {
			return nil, fmt.Errorf("misformatted user account id")
		}
		if userAccountIDStr != "" {
			id, err := uuid.Parse(userAccountIDStr)
			if err != nil {
				return nil, err
			}
			userAccountID = &id
		}
	}

	// i think this should try to find one if it exists

	newModel := model.Strategy{
		StrategyName:      name,
		FactorExpression:  expression,
		RebalanceInterval: rebalanceInterval,
		NumAssets:         int32(numAssets),
		AssetUniverse:     assetUniverse,
		UserAccountID:     userAccountID,
	}
	insertedStrategy, err := m.StrategyRepository.Add(newModel)
	if err != nil {
		return nil, err
	}

	return insertedStrategy, nil
}
