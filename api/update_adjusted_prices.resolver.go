package api

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

func (m ApiHandler) updateAdjustedPrices(c *gin.Context) {
	if m.AdjustedPriceService == nil {
		returnErrorJson(fmt.Errorf("adjusted price service is not configured"), c)
		return
	}

	symbols, err := m.rawPriceSymbols(c.Query("symbols"))
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	dryRun := strings.EqualFold(c.Query("dryRun"), "true")
	full := strings.EqualFold(c.Query("full"), "true")
	result, err := m.AdjustedPriceService.UpdateAdjustedPrices(c, symbols, dryRun, full)
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	c.JSON(200, result)
}
