package domain

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Portfolio struct {
	Positions map[string]*Position
	Cash      decimal.Decimal
}

func NewPortfolio() *Portfolio {
	return &Portfolio{
		Positions: map[string]*Position{},
		Cash:      decimal.Zero,
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

func (p Portfolio) TotalValue(priceMap map[string]decimal.Decimal) (decimal.Decimal, error) {
	totalValue := p.Cash
	for symbol, position := range p.Positions {
		price, ok := priceMap[symbol]
		if !ok {
			return decimal.Zero, fmt.Errorf("cannot compute portfolio total value: price map missing %s", symbol)
		}
		totalValue = totalValue.Add(position.ExactQuantity.Mul(price))
	}

	return totalValue, nil
}

type Position struct {
	Symbol   string
	Quantity float64
	// todo - migrate off quantity
	ExactQuantity decimal.Decimal
	TickerID      uuid.UUID
}

func (p Position) DeepCopy() *Position {
	return &Position{
		Symbol:        p.Symbol,
		Quantity:      p.Quantity,
		ExactQuantity: p.ExactQuantity,
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
	TickerID      uuid.UUID
	ExactQuantity decimal.Decimal
	ExpectedPrice float64
}

type ProposedTrades []ProposedTrade
