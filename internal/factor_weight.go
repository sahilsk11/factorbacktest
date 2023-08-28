package internal

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"

	"time"

	"github.com/montanaflynn/stats"
)

// Figure out how to weight portfolio given
// factor scores and (maybe) benchmark

/**

1. anchor portfolio
pick a sample portfolio. the factors will tilt the portfolio so that all of the original assets are still present. it will also tilt relative to the original weightings. this is a good way to see, "this is my portfolio, how would slightly adding a factor help"

2. num tickers
for every asset in the universe, compute the factor score. take the top N+1 assets. take the z-score of each (where the population is the top N+1 assets) and scale the scores between [0, 1]. We take N+1 because the lowest element will have scaled value of 0. assign each asset the weighted average of the scaled value.

3. top N-tile
for every asset in the universe, compute the factor score. take range of socres and figure out what the top N-tile of the scores is; specifically how many symbols it includes (picture a number line with all factor scores). make that value the num tickers for the iteration and then calculate via num tickers.


the biggest question is how to manage un-normalized factors. theoretically the factors can be anything. for example, if returns are 5%, 10% and 20% for two assets and i use (7 day rolling avg) vs  5*(7 day rolling avg), i would get [5, 10, 20] and [25, 50, 100].

A: score range: dist([5, 20]) = 15. 15/4 (what is a quarter of the distance) = 8.25. so the final quarter would be 20-8.25 = 12.25. so len(scores > 12.25) = 1
B: dist([25, 50]) = 75. 75/4 = 18.75. 100-18.75 = 71.25. len(scores > 71.25) = 1

in this case it's the same but be careful of weird equations where it might not be. i think if all factors are scaled the same way then it's fine.

*/

type AssetSelectionMode string

const (
	// always have N tickers in portfolio
	AssetSelectionMode_NumTickers AssetSelectionMode = "NUM_SYMBOLS"
	// use start portfolio as anchor
	AssetSelectionMode_AnchorPortfolio AssetSelectionMode = "ANCHOR_PORTFOLIO"
	// not even top X assets - only keep the highest
	// assets of the factor scores. AKA variable num tickers
	// basically sum all the factor scores
	AssetSelectionMode_TopQuartile AssetSelectionMode = "TOP_QUARTILE"
)

func NewAssetSelectionMode(s string) (*AssetSelectionMode, error) {
	m := map[string]AssetSelectionMode{
		"NUM_SYMBOLS":      AssetSelectionMode_NumTickers,
		"ANCHOR_PORTFOLIO": AssetSelectionMode_AnchorPortfolio,
		"TOP_QUARTILE":     AssetSelectionMode_TopQuartile,
	}
	for k, v := range m {
		if strings.EqualFold(
			strings.ReplaceAll(k, "_", ""),
			strings.ReplaceAll(s, "_", ""),
		) {
			return &v, nil
		}
	}
	return nil, fmt.Errorf("could not convert '%s' to known asset selection mode", s)
}

type AssetSelectionOptions struct {
	NumTickers             *int
	AnchorPortfolioWeights map[string]float64
	Mode                   AssetSelectionMode
}

func (aso AssetSelectionOptions) Valid() error {
	// TODO - add more checks
	prefix := fmt.Sprintf("asset selection mode is %s", aso.Mode)
	switch aso.Mode {
	case AssetSelectionMode_NumTickers:
		if aso.NumTickers == nil {
			return fmt.Errorf(prefix + " and num tickers is nil")
		}
		if *aso.NumTickers == 0 {
			return fmt.Errorf(prefix + " and num tickers is 0")
		}
	case AssetSelectionMode_AnchorPortfolio:
		if aso.AnchorPortfolioWeights == nil {
			return fmt.Errorf(prefix + " and anchor portfolio is nil")
		}
		if len(aso.AnchorPortfolioWeights) < 2 {
			return fmt.Errorf(prefix + " and anchor portfolio has < 2 assets")
		}
		sum := 0.0
		for _, a := range aso.AnchorPortfolioWeights {
			sum += a
		}
		if math.Abs(1-sum) > 0.0001 {
			return fmt.Errorf("sum of anchor portfolio weights sums to %f", sum)
		}
	case AssetSelectionMode_TopQuartile:
		return nil
	default:
		return fmt.Errorf("unknown asset selection mode '%s'", aso.Mode)
	}
	return nil
}

type CalculateTargetAssetWeightsInput struct {
	Tx                    *sql.Tx
	Date                  time.Time
	FactorScoresBySymbol  map[string]float64
	FactorIntensity       float64
	AssetSelectionOptions AssetSelectionOptions
}

// use a factor strategy and determine what the weight
// of each asset should be on given date
func CalculateTargetAssetWeights(in CalculateTargetAssetWeightsInput) (map[string]float64, error) {
	if in.FactorIntensity <= 0 || in.FactorIntensity > 1 {
		return nil, fmt.Errorf("factor intensity must be between (0, 1], got %f", in.FactorIntensity)
	}
	// todo - validate rest of input
	// keys(FactorScoresBySymbol) == keys(Benchmark)
	// non-nil maps, valid date

	err := in.AssetSelectionOptions.Valid()
	if err != nil {
		return nil, fmt.Errorf("failed to verify asset selection options: %w", err)
	}

	var newWeights map[string]float64

	switch in.AssetSelectionOptions.Mode {
	case AssetSelectionMode_NumTickers:
		newWeights, err = calculateWeightsViaNumTickers(
			*in.AssetSelectionOptions.NumTickers,
			in.FactorScoresBySymbol,
		)
	case AssetSelectionMode_AnchorPortfolio:
		newWeights, err = calculateWeightsRelativeToAnchor(
			in.AssetSelectionOptions.AnchorPortfolioWeights,
			in.FactorScoresBySymbol,
			in.FactorIntensity,
		)
	case AssetSelectionMode_TopQuartile:
		newWeights = map[string]float64{}
	default:
		return nil, fmt.Errorf("unknown asset selection '%s'", in.AssetSelectionOptions.Mode)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to calculate weights for mode %s: %w", in.AssetSelectionOptions.Mode, err)
	}

	// validate new weights add to 100
	sum := 0.0
	for _, w := range newWeights {
		sum += w
	}
	if math.Abs(sum-1) > 0.0001 {
		return nil, fmt.Errorf("new weight should sum to 1, got %f", sum)
	}

	return newWeights, nil
}

func zScoreBySymbol(factorScoreBySymbol map[string]float64) (map[string]float64, error) {
	if len(factorScoreBySymbol) < 2 {
		return nil, fmt.Errorf("cannot compute z-score of less than two values, got %d value(s)", len(factorScoreBySymbol))
	}
	dataset := []float64{}
	for _, factorScore := range factorScoreBySymbol {
		dataset = append(dataset, factorScore)
	}
	mean, err := stats.Mean(dataset)
	if err != nil {
		return nil, err
	}
	stdev, err := stats.StandardDeviationSample(dataset)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate stdev: %w", err)
	}

	zScoreBySymbol := map[string]float64{}
	for symbol, factorScore := range factorScoreBySymbol {
		zScore := (factorScore - mean) / stdev
		zScoreBySymbol[symbol] = zScore
	}

	return zScoreBySymbol, nil
}

// Peter's fancy math of calculating new weights
// based on linear weighting of factor score z-
// score
func calculateWeightsRelativeToAnchor(
	anchorPortfolioWeights map[string]float64,
	factorScoresBySymbol map[string]float64,
	factorIntensity float64,
) (map[string]float64, error) {
	zScoreBySymbol, err := zScoreBySymbol(factorScoresBySymbol)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate z score for factor scores: %w", err)
	}

	// to ensure asset weights do not drop below 0 or over 100,
	// we maintain the max scale factor that the assets can be
	// weighted by
	maxScaleFactor := 1.0

	for symbol, zScore := range zScoreBySymbol {
		maxB := (1 - anchorPortfolioWeights[symbol]) / zScore
		if zScore < 0 {
			maxB = anchorPortfolioWeights[symbol] / -zScore
		}
		if maxB < maxScaleFactor {
			maxScaleFactor = maxB
		}
	}

	scaleFactor := maxScaleFactor * factorIntensity

	newWeights := map[string]float64{}
	for symbol, originalWeight := range anchorPortfolioWeights {
		// w' = w0 + k * z_score
		newWeights[symbol] = originalWeight + scaleFactor*zScoreBySymbol[symbol]
	}

	return newWeights, nil
}

func calculateWeightsViaNumTickers(
	numTickers int,
	factorScoresBySymbol map[string]float64,
) (map[string]float64, error) {
	topScores := topNScores(factorScoresBySymbol, numTickers)
	numTickers = len(topScores)
	if numTickers == 1 {
		for symbol := range topScores {
			return map[string]float64{
				symbol: 1,
			}, nil
		}
	}
	originalWeights := map[string]float64{}
	for symbol := range topScores {
		originalWeights[symbol] = 1.0 / float64(numTickers)
	}

	return calculateWeightsRelativeToAnchor(
		originalWeights,
		topScores,
		0.999,
	)
}

func topNScores(factorScoresBySymbol map[string]float64, n int) map[string]float64 {
	var keyValuePairs []struct {
		Key   string
		Value float64
	}

	for key, value := range factorScoresBySymbol {
		keyValuePairs = append(keyValuePairs, struct {
			Key   string
			Value float64
		}{key, value})
	}

	sort.Slice(keyValuePairs, func(i, j int) bool {
		return keyValuePairs[i].Value > keyValuePairs[j].Value
	})

	if len(keyValuePairs) > n {
		keyValuePairs = keyValuePairs[:n]
	}

	topNMap := make(map[string]float64)
	for _, kv := range keyValuePairs {
		topNMap[kv.Key] = kv.Value
	}

	return topNMap
}
