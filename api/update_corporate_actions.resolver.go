package api

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

type UpdateCorporateActionsResponse struct {
	NumRows       int      `json:"numRows"`
	FailedSymbols []string `json:"failedSymbols"`
	Source        string   `json:"source"`
}

func (m ApiHandler) updateCorporateActions(c *gin.Context) {
	if m.CorporateActionService == nil {
		returnErrorJson(fmt.Errorf("corporate action service is not configured"), c)
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

	result, err := m.CorporateActionService.IngestCorporateActions(c, symbols, start, end, c.Query("source"))
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	c.JSON(200, UpdateCorporateActionsResponse{
		NumRows:       result.NumRows,
		FailedSymbols: result.FailedSymbols,
		Source:        result.Source,
	})
}
