package integration_tests

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"factorbacktest/api"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gocarina/gocsv"
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
	models := []model.Universe{
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
	query := table.Universe.INSERT(table.Universe.MutableColumns).MODELS(models)
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
	// db, err := internal.NewTestDb()
	// require.NoError(t, err)
	// tx, err := db.Begin()
	// require.NoError(t, err)
	// defer tx.Rollback()

	// // seed data
	// err = seedUniverse(tx)
	// require.NoError(t, err)
	// err = seedPrices(tx)
	// require.NoError(t, err)

	// // fml
	// err = tx.Commit()
	// require.NoError(t, err)

	/*
		{"factorOptions":{"expression":"pricePercentChange(\n  nDaysAgo(7),\n  currentDate\n) ","name":"7_day_momentum","intensity":0.75},"backtestStart":"2024-04-07","backtestEnd":"2024-06-02","samplingIntervalUnit":"weekly","startCash":10000,"anchorPortfolioQuantities":{"AAPL":10,"MSFT":15,"GOOGL":8},"assetSelectionMode":"NUM_SYMBOLS","numSymbols":10,"userID":"84c1c4de-2dbd-4c0e-84d5-830894d01b68"}*/

	numSymbols := 10
	startTime := time.Now()
	request := api.BacktestRequest{
		FactorOptions: struct {
			Expression string  "json:\"expression\""
			Intensity  float64 "json:\"intensity\""
			Name       string  "json:\"name\""
		}{
			Expression: "pricePercentChange(\n  nDaysAgo(7),\n  currentDate\n) ",
			Intensity:  0.75,
			Name:       "7_day_momentum",
		},
		BacktestStart:             "2020-01-01",
		BacktestEnd:               "2020-01-31",
		SamplingIntervalUnit:      "weekly",
		AssetSelectionMode:        "NUM_SYMBOLS",
		StartCash:                 10000,
		AnchorPortfolioQuantities: map[string]float64{},
		NumSymbols:                &numSymbols,
		UserID:                    nil,
	}
	response := api.BacktestResponse{}
	err := hitEndpoint("backtest", http.MethodPost, request, &response)
	require.NoError(t, err)
	elapsed := time.Since(startTime).Milliseconds()

	fmt.Println(response)

	require.Less(t, elapsed, int64(25e3))
}
