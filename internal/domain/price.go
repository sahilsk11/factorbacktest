package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type AssetPrice struct {
	Symbol string
	Price  decimal.Decimal
	Date   time.Time
}
