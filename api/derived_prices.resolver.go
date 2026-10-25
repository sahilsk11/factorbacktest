package api

import (
	"fmt"
	"time"

	"factorbacktest/internal/data"

	"github.com/gin-gonic/gin"
)

type UpdateDerivedPricesResponse struct {
	Raw                  *UpdateRawPricesResponse        `json:"raw,omitempty"`
	CorporateActions     *UpdateCorporateActionsResponse `json:"corporateActions,omitempty"`
	CorporateActionError string                          `json:"corporateActionError,omitempty"`
	FullRecompute        []string                        `json:"fullRecompute"`
	Appended             []string                        `json:"appended"`
	FailedSymbols        []string                        `json:"failedSymbols"`
	Wrote                bool                            `json:"wrote"`
	NumRows              int                             `json:"numRows"`
}

// updateDerivedPrices runs the derived-price daily flow, in order:
// (1) ingest raw closes, (2) ingest corporate actions, (3) fully recompute
// adjusted history for symbols with a recently ingested/effective action,
// acquired/delisted names, or no existing adjusted rows, (4) append/upsert
// only the latest day for everyone else. Writes to adjusted_price only when
// write=true|1. crontab and fly.toml are unchanged; run this after hours.
func (m ApiHandler) updateDerivedPrices(c *gin.Context) {
	if m.DerivedPriceService == nil {
		returnErrorJson(fmt.Errorf("derived price service is not configured"), c)
		return
	}
	start, err := parseOptionalDateQuery(c.Query("start"))
	if err != nil {
		returnErrorJsonCode(err, c, 400)
		return
	}
	end, err := parseOptionalDateQuery(c.Query("end"))
	if err != nil {
		returnErrorJsonCode(err, c, 400)
		return
	}
	symbols, err := m.rawPriceSymbols(c.Query("symbols"))
	if err != nil {
		returnErrorJson(err, c)
		return
	}
	result, err := m.DerivedPriceService.UpdateDerivedPrices(c, data.DerivedUpdateInput{
		Symbols: symbols, Start: start, End: end,
		Write:  c.Query("write") == "true" || c.Query("write") == "1",
		Ingest: c.Query("ingest") != "false" && c.Query("ingest") != "0",
		Full:   c.Query("full") == "true" || c.Query("full") == "1",
	})
	if err != nil {
		returnErrorJson(err, c)
		return
	}
	out := UpdateDerivedPricesResponse{
		CorporateActionError: result.CorporateActionErr,
		FullRecompute:        result.FullRecompute, Appended: result.Appended,
		FailedSymbols: result.FailedSymbols, Wrote: result.Wrote, NumRows: result.NumRows,
	}
	if result.Raw != nil {
		out.Raw = &UpdateRawPricesResponse{NumUpdatedAssets: len(result.Raw.UpdatedSymbols), FailedSymbols: result.Raw.FailedSymbols, Source: result.Raw.Source}
	}
	if result.CorporateActions != nil {
		out.CorporateActions = &UpdateCorporateActionsResponse{NumRows: result.CorporateActions.NumRows, FailedSymbols: result.CorporateActions.FailedSymbols, Source: result.CorporateActions.Source}
	}
	c.JSON(200, out)
}

type VerifyAdjustedPriceRow struct {
	Symbol         string   `json:"symbol"`
	ComputedPoints int      `json:"computedPoints"`
	Overlap        int      `json:"overlap"`
	MaxRelDiff     string   `json:"maxRelDiff"`
	MaxRelDiffDate string   `json:"maxRelDiffDate,omitempty"`
	LatestDate     string   `json:"latestDate,omitempty"`
	LatestFactor   string   `json:"latestFactor,omitempty"`
	LatestRaw      string   `json:"latestRaw,omitempty"`
	LatestComputed string   `json:"latestComputed,omitempty"`
	LatestStored   string   `json:"latestStored,omitempty"`
	Error          string   `json:"error,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
}

func (m ApiHandler) verifyAdjustedPrices(c *gin.Context) {
	if m.DerivedPriceService == nil {
		returnErrorJson(fmt.Errorf("derived price service is not configured"), c)
		return
	}
	symbols := append([]string{}, data.DefaultVerifySymbols...)
	if c.Query("symbols") != "" {
		var err error
		symbols, err = m.rawPriceSymbols(c.Query("symbols"))
		if err != nil {
			returnErrorJson(err, c)
			return
		}
	}
	diffs, err := m.DerivedPriceService.VerifyAdjustedPrices(c, symbols)
	if err != nil {
		returnErrorJson(err, c)
		return
	}
	rows := make([]VerifyAdjustedPriceRow, 0, len(diffs))
	for _, d := range diffs {
		row := VerifyAdjustedPriceRow{Symbol: d.Symbol, ComputedPoints: d.ComputedPoints, Overlap: d.Overlap, MaxRelDiff: d.MaxRelDiff.String(), Error: d.Err, Warnings: d.Warnings}
		if d.MaxRelDiffDate != nil {
			row.MaxRelDiffDate = d.MaxRelDiffDate.Format(time.DateOnly)
		}
		if d.Latest != nil {
			row.LatestDate = d.Latest.Date.Format(time.DateOnly)
			row.LatestFactor, row.LatestRaw, row.LatestComputed = d.Latest.Factor.String(), d.Latest.Raw.String(), d.Latest.Price.String()
		}
		if d.LatestStored != nil {
			row.LatestStored = d.LatestStored.String()
		}
		rows = append(rows, row)
	}
	c.JSON(200, gin.H{"symbols": rows})
}
