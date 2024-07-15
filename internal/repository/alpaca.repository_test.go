package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
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
	}, nil
}

func Test_alpacaRepositoryHandler_GetAccount(t *testing.T) {
	if true {
		t.Skip()
	}

	handler, err := initializeHandler()
	require.NoError(t, err)
	defer func() {
		handler.CancelOpenOrders()
	}()

	order, err := handler.PlaceOrder(AlpacaPlaceOrderRequest{
		TradeOrderID:    uuid.New(),
		AmountInDollars: decimal.NewFromInt(12),
		Symbol:          "AAPL",
		Side:            alpaca.Buy,
	})
	require.NoError(t, err)

	fmt.Println(order.Status)
	fmt.Println(order.ID)
	// fmt.Println(order.)

	// fmt.Println(order.AssetClass)
	// fmt.Println(order.FailedAt)
	fmt.Println("P", order.FilledAvgPrice)
	fmt.Println("q", order.FilledQty)
	fmt.Println("q", order.FilledAt)
	fmt.Println("n", order.Notional)

	time.Sleep(3 * time.Second)
	t.Fail()
}
