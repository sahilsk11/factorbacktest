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

type LatestExcessTradeVolume struct {
	ExcessTradeVolumeID *uuid.UUID
	TickerID            *uuid.UUID
	Symbol              *string
	Quantity            *decimal.Decimal
	RebalancerRunID     *uuid.UUID
	CreatedAt           *time.Time
}