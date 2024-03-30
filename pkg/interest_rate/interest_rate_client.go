package interestrate

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
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

type InterestRateMap struct {
	Rates map[int]float64
}

func (im InterestRateMap) GetRate(monthsOut int) float64 {
	v, ok := im.Rates[monthsOut]
	if ok {
		return v
	}

	keys := []int{}
	for k := range im.Rates {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})

	// figure out closest values and interpolate
	if monthsOut < keys[0] {
		return im.Rates[keys[0]]
	}
	if monthsOut > keys[len(keys)-1] {
		return im.Rates[keys[len(keys)-1]]
	}

	for i := 0; i < len(keys)-1; i++ {
		key1 := keys[i]
		key2 := keys[i+1]
		if monthsOut > key1 && monthsOut < key2 {
			return (im.Rates[key1] + im.Rates[key2]) / 2
		}
	}
	panic("unable to compute rate")
}

/*

{
	oneMonth: {
		hoursFromToday: N,
		rate: K,
	}
}



*/

func GetYieldCurve(date time.Time) (*InterestRateMap, error) {
	client := http.DefaultClient

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

	tStr := date.Format(time.DateOnly)
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

	responseBody := []map[string]interface{}{}

	err = json.Unmarshal(responseBytes, &responseBody)
	if err != nil {
		return nil, err
	}

	out := map[int]float64{}

	for _, response := range responseBody {
		for k, v := range response {
			for _, field := range keys {
				if k == field {
					// TODO - if field is null, interpolate between values
					months, err := interestRateMonthsFromApi(k)
					if err != nil {
						return nil, err
					}
					if v != nil {
						out[months] = v.(float64) / 100
					}
				}
			}
		}
	}

	return &InterestRateMap{
		Rates: out,
	}, nil
}
