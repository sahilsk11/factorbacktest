package api

import (
	"factorbacktest/internal/repository"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type GetInvestmentsResponse struct {
	StrategyInvestmentID uuid.UUID `json:"strategyInvestmentID"`
	AmountDollars        int32     `json:"amountDollars"`
	StartDate            time.Time `json:"startDate"`
	SavedStragyID        uuid.UUID `json:"savedStrategyID"`
	UserAccountID        uuid.UUID `json:"userAccountID"`
	CreatedAt            time.Time `json:"createdAt"`
}

func (m ApiHandler) getInvestments(c *gin.Context) {
	ginUserAccountID, ok := c.Get("userAccountID")
	if !ok {
		returnErrorJson(fmt.Errorf("must be logged in to save strategy"), c)
		return
	}
	userAccountIDStr, ok := ginUserAccountID.(string)
	if !ok {
		returnErrorJson(fmt.Errorf("misformatted user account id"), c)
		return
	}

	userAccountID, err := uuid.Parse(userAccountIDStr)
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	investments, err := m.InvestmentRepository.List(repository.StrategyInvestmentListFilter{
		UserAccountIDs: []uuid.UUID{userAccountID},
	})
	if err != nil {
		returnErrorJson(err, c)
		return
	}

	out := []GetInvestmentsResponse{}
	for _, i := range investments {
		out = append(out, GetInvestmentsResponse{
			StrategyInvestmentID: i.InvestmentID,
			AmountDollars:        i.AmountDollars,
			StartDate:            i.StartDate,
			SavedStragyID:        i.SavedStragyID,
			UserAccountID:        i.UserAccountID,
			CreatedAt:            i.CreatedAt,
		})
	}

	c.JSON(200, out)
}
