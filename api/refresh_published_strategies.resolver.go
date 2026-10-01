package api

import (
	"context"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/logger"
	"factorbacktest/internal/repository"
	"factorbacktest/internal/util"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

type refreshPublishedStrategiesResponse struct {
	Status    string   `json:"status"`
	Refreshed int      `json:"refreshed,omitempty"`
	Failed    []string `json:"failed,omitempty"`
}

func (m ApiHandler) refreshPublishedStrategies(c *gin.Context) {
	lg := logger.FromContext(c)
	// Work continues after the HTTP response so the Fly proxy's ~60s
	// timeout cannot cancel a multi-strategy 3-year backtest. supercronic
	// treats a 200 as success and will not retry the job.
	go func() {
		ctx := context.WithValue(context.Background(), logger.ContextKey, lg)
		profile, endProfile := domain.NewProfile()
		defer endProfile()
		ctx = context.WithValue(ctx, domain.ContextProfileKey, profile)
		result := m.refreshPublishedStrategyRuns(ctx)
		lg.Infow(
			"published strategy refresh finished",
			"status", result.Status,
			"refreshed", result.Refreshed,
			"failed", result.Failed,
		)
	}()
	c.JSON(200, refreshPublishedStrategiesResponse{Status: "started"})
}

func (m ApiHandler) refreshPublishedStrategyRuns(ctx context.Context) refreshPublishedStrategiesResponse {
	lg := logger.FromContext(ctx)
	out := refreshPublishedStrategiesResponse{Status: "ok"}

	strategies, err := m.StrategyRepository.List(repository.StrategyListFilter{
		Published: util.BoolPointer(true),
	})
	if err != nil {
		lg.Errorf("failed to list published strategies: %v", err)
		return refreshPublishedStrategiesResponse{
			Status: "error",
			Failed: []string{err.Error()},
		}
	}

	start, end := publishedBacktestWindow(time.Now().UTC())
	for _, strategy := range strategies {
		if err := m.refreshOnePublishedStrategy(ctx, strategy, start, end); err != nil {
			lg.Errorf("failed to refresh published strategy %s (%s): %v", strategy.StrategyID, strategy.StrategyName, err)
			out.Failed = append(out.Failed, fmt.Sprintf("%s: %v", strategy.StrategyName, err))
			continue
		}
		out.Refreshed++
	}
	return out
}

func (m ApiHandler) refreshOnePublishedStrategy(
	ctx context.Context,
	strategy model.Strategy,
	start, end time.Time,
) error {
	resp, metrics, err := m.computeStrategyBacktest(ctx, strategy, start, end, publishedBacktestStartCash)
	if err != nil {
		return err
	}
	return m.persistPublishedStrategyRun(strategy, start, end, resp, metrics)
}
