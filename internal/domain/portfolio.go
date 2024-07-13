package domain

import (
	"fmt"
)

type Portfolio struct {
	Positions map[string]*Position
	Cash      float64
}

func NewPortfolio() *Portfolio {
	return &Portfolio{
		Positions: map[string]*Position{},
		Cash:      0,
	}
}

func (p Portfolio) DeepCopy() *Portfolio {
	newPortfolio := &Portfolio{
		Cash:      p.Cash,
		Positions: map[string]*Position{},
	}
	for symbol, position := range p.Positions {
		newPortfolio.Positions[symbol] = position.DeepCopy()
	}

	return newPortfolio
}

func (p Portfolio) TotalValue(priceMap map[string]float64) (float64, error) {
	totalValue := p.Cash
	for symbol, position := range p.Positions {
		price, ok := priceMap[symbol]
		if !ok {
			return 0, fmt.Errorf("cannot compute portfolio total value: price map missing %s", symbol)
		}
		totalValue += position.Quantity * price
	}

	return totalValue, nil
}

func (p Portfolio) AssetWeightsExcludingCash(priceMap map[string]float64) (map[string]float64, error) {
	totalValue, err := p.TotalValue(priceMap)
	if err != nil {
		return nil, err
	}

	totalValueExcludingCash := totalValue - p.Cash
	weights := map[string]float64{}
	for symbol, position := range p.Positions {
		// gonna assume that price map has symbol if prev call
		// didn't fail
		// TODO - check if symbol is missing
		weights[symbol] = position.Quantity * priceMap[symbol] / totalValueExcludingCash
	}

	return weights, nil
}

type Position struct {
	Symbol   string
	Quantity float64
}

func (p Position) DeepCopy() *Position {
	return &Position{
		Symbol:   p.Symbol,
		Quantity: p.Quantity,
	}
}

// stupid func
func PositionsFromQuantity(in map[string]float64) map[string]*Position {
	positions := map[string]*Position{}
	for symbol, quantity := range in {
		positions[symbol] = &Position{
			Symbol:   symbol,
			Quantity: quantity,
		}
	}
	return positions
}

type ProposedTrade struct {
	Symbol        string
	Quantity      float64 // negative is valid and implies sell
	ExpectedPrice float64
}

type ProposedTrades []ProposedTrade
