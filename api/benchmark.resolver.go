package api

import (
	"context"
	"database/sql"
	"time"

	"github.com/gin-gonic/gin"
)

type benchmarkResponse map[string]float64

type benchmarkRequest struct {
	Symbol      string `json:"symbol"`
	Start       string `json:"start"`
	End         string `json:"end"`
	Granularity string `json:"granularity"`
}

func (h ApiHandler) benchmark(c *gin.Context) {
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
	defer tx.Rollback()

	var requestBody benchmarkRequest

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		returnErrorJson(err, c)
		return
	}

	start, err := time.Parse("2006-01-02", requestBody.Start)
	if err != nil {
		returnErrorJson(err, c)
		return
	}
	end, err := time.Parse("2006-01-02", requestBody.End)
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	granularity := time.Hour * 24
	if requestBody.Granularity == "weekly" {
		granularity *= 7
	} else if requestBody.Granularity == "monthly" {
		granularity *= 30
	}

	results, err := h.BenchmarkHandler.GetIntraPeriodChange(
		tx,
		requestBody.Symbol,
		start,
		end,
		granularity,
	)
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	out := benchmarkResponse{}
	for k, v := range results {
		out[k.Format("2006-01-02")] = v
	}

	c.JSON(200, out)
}
