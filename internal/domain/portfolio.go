package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Portfolio struct {
	Positions map[string]*Position
	Cash      *decimal.Decimal // todo - idt this needs to be a decimal? check
}

func NewPortfolio() *Portfolio {
	d := decimal.Zero
	return &Portfolio{
		Positions: map[string]*Position{},
		Cash:      &d,
	}
}

func (p Portfolio) HeldSymbols() []string {
	symbols := []string{}
	for symbol := range p.Positions {
		symbols = append(symbols, symbol)
	}
	return symbols
}

func (p *Portfolio) SetCash(newCash decimal.Decimal) {
	p.Cash = &newCash
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
	totalValue := *p.Cash
	missing := p.UnpricedSymbols(priceMap)
	for symbol, position := range p.Positions {
		price, ok := priceMap[symbol]
		if !ok || !price.GreaterThan(decimal.Zero) {
			continue
		}
		totalValue = totalValue.Add(position.ExactQuantity.Mul(price))
	}
	if len(missing) > 0 {
		return totalValue, fmt.Errorf("price map missing %s", strings.Join(missing, ", "))
	}

	return totalValue, nil
}

// UnpricedSymbols lists held names with no usable (present and positive) price.
// Missing is not treated as zero; callers should skip sizing those names.
func (p Portfolio) UnpricedSymbols(priceMap map[string]decimal.Decimal) []string {
	missing := make([]string, 0)
	for symbol := range p.Positions {
		price, ok := priceMap[symbol]
		if !ok || !price.GreaterThan(decimal.Zero) {
			missing = append(missing, symbol)
		}
	}
	sort.Strings(missing)
	return missing
}

type Position struct {
	Symbol   string
	Quantity float64
	// todo - migrate off quantity
	ExactQuantity decimal.Decimal
	TickerID      uuid.UUID

	// we're getting to a point where the models are overloaded
	// this may not be set in some places
	Value *decimal.Decimal
}

func (p Position) DeepCopy() *Position {
	return &Position{
		Symbol:        p.Symbol,
		Quantity:      p.Quantity,
		ExactQuantity: p.ExactQuantity,
		TickerID:      p.TickerID,
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
	ExpectedPrice decimal.Decimal
}

func (p ProposedTrade) ExpectedAmount() decimal.Decimal {
	return p.ExactQuantity.Mul(p.ExpectedPrice).Abs()
}

type FilledTrade struct {
	Symbol    string
	TickerID  uuid.UUID
	Quantity  decimal.Decimal
	FillPrice decimal.Decimal
	FilledAt  time.Time
}
