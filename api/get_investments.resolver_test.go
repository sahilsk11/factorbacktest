package api

import (
	"factorbacktest/internal/service"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestGetInvestmentsResponseFromDomainIncludesLiquidationRequestedAt(t *testing.T) {
	investmentID := uuid.New()
	requestedAt := time.Date(2026, 7, 14, 15, 30, 0, 0, time.UTC)

	out := getInvestmentsResponseFromDomain(map[uuid.UUID]service.GetStatsResponse{
		investmentID: {
			StartDate:              time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC),
			LiquidationRequestedAt: &requestedAt,
		},
	})

	require.Len(t, out, 1)
	require.NotNil(t, out[0].LiquidationRequestedAt)
	require.Equal(t, "2026-07-14T15:30:00Z", *out[0].LiquidationRequestedAt)
}
