package internal

import (
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
the biggest question is how to manage un-normalized factors. theoretically the factors can be anything. for example, if returns are 5%, 10% and 20% for two assets and i use (7 day rolling avg) vs  5*(7 day rolling avg), i would get [5, 10, 20] and [25, 50, 100].

A: score range: dist([5, 20]) = 15. 15/4 (what is a quarter of the distance) = 8.25. so the final quarter would be 20-8.25 = 12.25. so len(scores > 12.25) = 1
B: dist([25, 50]) = 75. 75/4 = 18.75. 100-18.75 = 71.25. len(scores > 71.25) = 1

in this case it's the same but be careful of weird equations where it might not be. i think if all factors are scaled the same way then it's fine.
*/

type CalculateTargetAssetWeightsInput struct {
	Date                 time.Time
	FactorScoresBySymbol map[string]*float64
	NumTickers           int
}

// use a factor strategy and determine what the weight
// of each asset should be on given date
func CalculateTargetAssetWeights(in CalculateTargetAssetWeightsInput) (map[string]float64, error) {
	// todo - validate rest of input
	// keys(FactorScoresBySymbol) == keys(Benchmark)
	// non-nil maps, valid date

	newWeights, err := calculateWeightsViaNumTickers(
		in.NumTickers,
		in.FactorScoresBySymbol,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate weights: %w", err)
	}

	// validate new weights add to 100
	sum := 0.0
	for symbol, w := range newWeights {
		if math.IsNaN(w) {
			return nil, fmt.Errorf("invalid weight NaN for %s", symbol)
		}
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
	if stdev == 0 {
		return nil, fmt.Errorf("0 stdev")
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
	if err != nil && strings.Contains(err.Error(), "0 stdev") {
		return anchorPortfolioWeights, nil
	} else if err != nil {
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
	factorScoresBySymbol map[string]*float64,
) (map[string]float64, error) {
	topScores := topNScores(factorScoresBySymbol, numTickers)
	if len(topScores) != numTickers {
		return nil, fmt.Errorf("target portfolio should have %d assets but calculated scores for %d assets", numTickers, len(topScores))
	}
	numTickers = len(topScores)
	if numTickers == 1 {
		for symbol := range topScores {
			return map[string]float64{
				symbol: 1,
			}, nil
		}
	}
	// assign equal weighting to all assets to start
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

func topNScores(factorScoresBySymbol map[string]*float64, n int) map[string]float64 {
	var keyValuePairs []struct {
		Key   string
		Value float64
	}

	for key, value := range factorScoresBySymbol {
		if value != nil {
			keyValuePairs = append(keyValuePairs, struct {
				Key   string
				Value float64
			}{key, *value})
		}
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
