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

type SavedStrategy struct {
	SavedStragyID     uuid.UUID `sql:"primary_key"`
	StrategyName      string
	FactorExpression  string
	BacktestStart     time.Time
	BacktestEnd       time.Time
	RebalanceInterval string
	NumAssets         int32
	AssetUniverse     string
	Bookmarked        bool
	UserAccountID     uuid.UUID
	CreatedAt         time.Time
	ModifiedAt        time.Time
}
