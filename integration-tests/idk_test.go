package integration_tests

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"factorbacktest/api"
	"factorbacktest/internal"
	"factorbacktest/internal/app"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/gocarina/gocsv"
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func seedPrices(tx *sql.Tx) error {
	f, err := os.Open("sample_prices_2020.csv")
	if err != nil {
		return err
	}
	defer f.Close()

	type Row struct {
		Date   string  `csv:"date"`
		Symbol string  `csv:"symbol"`
		Price  float64 `csv:"price"`
	}
	rows := []Row{}
	gocsv.UnmarshalFile(f, &rows)

	models := []model.AdjustedPrice{}
	for _, row := range rows {
		date, err := time.Parse(time.DateOnly, row.Date)
		if err != nil {
			return err
		}
		models = append(models, model.AdjustedPrice{
			Date:   date,
			Symbol: row.Symbol,
			Price:  row.Price,
		})
	}

	query := table.AdjustedPrice.INSERT(table.AdjustedPrice.MutableColumns).MODELS(models)
	_, err = query.Exec(tx)
	if err != nil {
		return err
	}

	return nil
}

func seedUniverse(tx *sql.Tx) error {
	models := []model.Ticker{
		{
			Symbol: "AAPL",
			Name:   "Apple",
		},
		{
			Symbol: "GOOG",
			Name:   "Google",
		},
		{
			Symbol: "META",
			Name:   "Meta",
		},
	}
	query := table.Ticker.INSERT(table.Ticker.MutableColumns).MODELS(models)
	_, err := query.Exec(tx)
	if err != nil {
		return err
	}

	return nil
}

func hitEndpoint(route string, method string, payload interface{}, target interface{}) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	body := bytes.NewReader(payloadBytes)

	// Create the POST request
	req, err := http.NewRequest(method, "http://localhost:3009/"+route, body)
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
	// setup db
	db, err := internal.NewTestDb()
	require.NoError(t, err)
	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback()

	// seed data
	err = seedUniverse(tx)
	require.NoError(t, err)
	defer func() {
		_, err = table.Ticker.DELETE().WHERE(postgres.Bool(true)).Exec(db)
		require.NoError(t, err)
	}()
	err = seedPrices(tx)
	require.NoError(t, err)
	defer func() {
		_, err = table.AdjustedPrice.DELETE().WHERE(postgres.Bool(true)).Exec(db)
		require.NoError(t, err)
	}()

	err = tx.Commit()
	require.NoError(t, err)

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
	err = hitEndpoint("backtest", http.MethodPost, request, &response)
	require.NoError(t, err)
	elapsed := time.Since(startTime).Milliseconds()

	require.Equal(t, 51, len(response.Snapshots))
	require.Equal(
		t,
		"",
		cmp.Diff(
			app.BacktestSnapshot{
				ValuePercentChange: 33.6989043,
				Value:              13369.89043,
				Date:               "2020-12-29",
				AssetMetrics: map[string]app.ScnapshotAssetMetrics{
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
