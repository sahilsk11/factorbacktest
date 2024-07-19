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

	"github.com/shopspring/decimal"
)

type InvestmentTrade struct {
	InvestmentTradeID uuid.UUID `sql:"primary_key"`
	TickerID          uuid.UUID
	Side              TradeOrderSide
	CreatedAt         time.Time
	InvestmentID      uuid.UUID
	RebalancerRunID   uuid.UUID
	Quantity          decimal.Decimal
	TradeOrderID      *uuid.UUID
	ModifiedAt        time.Time
}
