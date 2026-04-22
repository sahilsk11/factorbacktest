package integration_tests

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"factorbacktest/api"
	"factorbacktest/internal/service"
	"factorbacktest/internal/testseed"
	"fmt"
	"io"
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func hitEndpoint(baseURL, route string, method string, payload interface{}, target interface{}) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	body := bytes.NewReader(payloadBytes)

	// Create the POST request
	req, err := http.NewRequest(method, baseURL+"/"+route, body)
	if err != nil {
		return err
	}

	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}

	// Send the request
	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read the response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	type ErrorResponse struct {
		Error string `json:"error"`
	}

	errResponse := ErrorResponse{}
	err = json.Unmarshal(responseBody, &errResponse)
	if err != nil {
		return err
	}

	if errResponse.Error != "" {
		return fmt.Errorf("failed with response body: %s", string(responseBody))
	}

	// Unmarshal the JSON response into the struct
	err = json.Unmarshal(responseBody, target)
	if err != nil {
		return err
	}

	return nil
}

func Test_backtestFlow(t *testing.T) {
	manager, err := NewTestDbManager()
	require.NoError(t, err)

	defer manager.Close()

	server, err := NewTestServer(manager)
	require.NoError(t, err)
	defer server.Stop()

	db := manager.DB()

	seed := func(db *sql.DB) {
		aapl := testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "AAPL", Name: "Apple"})
		goog := testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "GOOG", Name: "Google"})
		meta := testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "META", Name: "Meta"})
		universe := testseed.CreateAssetUniverse(db, testseed.AssetUniverseOpts{Name: "SPY_TOP_80"})
		for _, id := range []uuid.UUID{aapl.TickerID, goog.TickerID, meta.TickerID} {
			testseed.CreateAssetUniverseTicker(db, universe.AssetUniverseID, id)
		}
		testseed.InsertPrices2020(db)
	}
	seed(db)

	userID := uuid.NewString()
	startTime := time.Now()
	request := api.BacktestRequest{
		FactorOptions: struct {
			Expression string "json:\"expression\""
			Name       string "json:\"name\""
		}{
			Expression: "pricePercentChange(\n  nDaysAgo(7),\n  currentDate\n) ",
			Name:       "7_day_momentum",
		},
		BacktestStart:        "2020-01-10",
		BacktestEnd:          "2020-12-31",
		SamplingIntervalUnit: "weekly",
		StartCash:            10000,
		NumSymbols:           3,
		UserID:               &userID,
	}
	response := api.BacktestResponse{}
	err = hitEndpoint(server.URL, "backtest", http.MethodPost, request, &response)
	require.NoError(t, err)
	elapsed := time.Since(startTime).Milliseconds()

	require.Equal(t, 51, len(response.Snapshots))
	require.Equal(
		t,
		"",
		cmp.Diff(
			service.BacktestSnapshot{
				ValuePercentChange: 33.6989043,
				Value:              13369.88700,
				Date:               "2020-12-29",
				AssetMetrics: map[string]service.SnapshotAssetMetrics{
					"AAPL": {
						AssetWeight:                  0.1253766234821042,
						FactorScore:                  2.2169708025194654,
						PriceChangeTilNextResampling: nil,
					},
					"GOOG": {
						AssetWeight:                  0.00033333333333335213,
						FactorScore:                  2.0025859914157165,
						PriceChangeTilNextResampling: nil,
					},
					"META": {
						AssetWeight:                  0.8742900431845622,
						FactorScore:                  3.500971422024536,
						PriceChangeTilNextResampling: nil,
					},
				},
			},
			response.Snapshots["2020-12-29"],
			cmp.Comparer(func(i, j float64) bool {
				return math.Abs(i-j) < 1e-4
			}),
		),
	)

	// 1800 today
	require.Less(t, elapsed, int64(2500))
}
