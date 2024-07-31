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

type PublishedStrategy struct {
	PublishedStrategyID uuid.UUID `sql:"primary_key"`
	StrategyName        string
	FactorExpression    string
	RebalanceInterval   string
	NumAssets           int32
	AssetUniverse       string
	CreatorAccountID    uuid.UUID
	CreatedAt           time.Time
	ModifiedAt          time.Time
	DeletedAt           *time.Time
}
