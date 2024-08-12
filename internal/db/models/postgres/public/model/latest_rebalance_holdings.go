//
// Code generated by go-jet DO NOT EDIT.
//
// WARNING: Changes to this file may cause incorrect behavior
// and will be lost if the code is regenerated
//

package model

import (
	"github.com/google/uuid"
	"time"
)

type LatestRebalanceHoldings struct {
	InvestmentHoldingsID        *uuid.UUID
	InvestmentID                *uuid.UUID
	Symbol                      *string
	Quantity                    *float64
	PriceAtRebalance            *float64
	AmountAtRebalance           *float64
	CreatedAt                   *time.Time
	TickerID                    *uuid.UUID
	InvestmentHoldingsVersionID *uuid.UUID
	RebalancerRunID             *uuid.UUID
}
