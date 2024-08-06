package repository

import (
	"encoding/json"
	"factorbacktest/internal/util"
	"fmt"
	"os"
	"testing"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
	"github.com/stretchr/testify/require"
)

func initializeHandler() (*alpacaRepositoryHandler, error) {
	secretsFile := "../../secrets-dev.json"
	f, err := os.ReadFile(secretsFile)
	if err != nil {
		return nil, fmt.Errorf("could not open secrets-dev.json: %w", err)
	}

	type secrets struct {
		Alpaca struct {
			ApiKey    string `json:"apiKey"`
			ApiSecret string `json:"apiSecret"`
		} `json:"alpaca"`
	}

	s := secrets{}
	err = json.Unmarshal(f, &s)
	if err != nil {
		return nil, err
	}

	return &alpacaRepositoryHandler{
		Client: alpaca.NewClient(alpaca.ClientOpts{
			APIKey:     s.Alpaca.ApiKey,
			APISecret:  s.Alpaca.ApiSecret,
			BaseURL:    "https://paper-api.alpaca.markets",
			RetryLimit: 3,
		}),
		MdClient: marketdata.NewClient(marketdata.ClientOpts{
			APIKey:    s.Alpaca.ApiKey,
			APISecret: s.Alpaca.ApiSecret,
		}),
	}, nil
}

func Test_alpacaRepositoryHandler_GetAccount(t *testing.T) {
	if true {
		// t.Skip()
	}

	handler, err := initializeHandler()
	require.NoError(t, err)

	prices, err := handler.GetLatestPrices([]string{"UPS", "V"})
	require.NoError(t, err)
	util.Pprint(prices)

	t.Fail()
}
