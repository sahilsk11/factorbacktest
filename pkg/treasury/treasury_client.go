package treasury_client

import (
	"encoding/json"
	"factorbacktest/internal/domain"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func interestRateMonthsFromApi(in string) (int, error) {
	cleanedStr := strings.Replace(in, "yield_", "", 1)
	unit := string(cleanedStr[len(cleanedStr)-1])
	cleanedStr = cleanedStr[:len(cleanedStr)-1]
	months, err := strconv.Atoi(cleanedStr)
	if err != nil {
		return 0, err
	}

	if unit == "y" {
		months *= 12
	}

	return months, nil
}

// lazy, in-memory cache for API requests
var cache map[string][]byte = map[string][]byte{}

func getBytes(date time.Time) ([]byte, error) {
	tStr := date.Format(time.DateOnly)

	if out, ok := cache[tStr]; ok {
		return out, nil
	}

	client := http.DefaultClient
	url := fmt.Sprintf("https://www.ustreasuryyieldcurve.com/api/v1/yield_curve_snapshot?date=%s&offset=0", tStr)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	// var responseJson FinancialResponse
	responseBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("received status code %d and failed to read body: %w", response.StatusCode, err)
	}

	if response.StatusCode != 200 {
		return nil, fmt.Errorf("failed with status code %d: %s", response.StatusCode, string(responseBytes))
	}

	cache[tStr] = responseBytes

	return responseBytes, nil
}

func GetInterestRatesOnDay(date time.Time) (domain.InterestRateMap, error) {
	keys := []string{
		"yield_1m",
		"yield_2m",
		"yield_3m",
		"yield_4m",
		"yield_6m",
		"yield_1y",
		"yield_2y",
		"yield_3y",
		"yield_5y",
		"yield_7y",
		"yield_10y",
		"yield_20y",
		"yield_30y",
	}

	responseBytes, err := getBytes(date)
	if err != nil {
		return domain.InterestRateMap{}, err
	}

	responseBody := []map[string]interface{}{}

	err = json.Unmarshal(responseBytes, &responseBody)
	if err != nil {
		return domain.InterestRateMap{}, err
	}

	out := map[int]float64{}
	oneNonNil := false

	for _, response := range responseBody {
		for k, v := range response {
			for _, field := range keys {
				if k == field {
					// TODO - if field is null, interpolate between values
					months, err := interestRateMonthsFromApi(k)
					if err != nil {
						return domain.InterestRateMap{}, err
					}
					if v != nil {
						oneNonNil = true
						out[months] = v.(float64) / 100
					}
				}
			}
		}
	}
	if !oneNonNil {
		// if the values are all nil, go backwards until you get a hit
		return GetInterestRatesOnDay(date.AddDate(0, -1, 0))
	}

	return domain.InterestRateMap{
		Rates: out,
	}, nil
}
