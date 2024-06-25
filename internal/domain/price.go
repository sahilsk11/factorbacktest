package domain

import "time"

type AssetPrice struct {
	Symbol string
	Price  float64
	Date   time.Time
}
