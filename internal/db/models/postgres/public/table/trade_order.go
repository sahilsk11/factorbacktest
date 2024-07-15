//
// Code generated by go-jet DO NOT EDIT.
//
// WARNING: Changes to this file may cause incorrect behavior
// and will be lost if the code is regenerated
//

package table

import (
	"github.com/go-jet/jet/v2/postgres"
)

var TradeOrder = newTradeOrderTable("public", "trade_order", "")

type tradeOrderTable struct {
	postgres.Table

	//Columns
	TradeOrderID             postgres.ColumnString
	ProviderID               postgres.ColumnString
	TickerID                 postgres.ColumnString
	Side                     postgres.ColumnString
	RequestedAmountInDollars postgres.ColumnFloat
	Status                   postgres.ColumnString
	FilledQuantity           postgres.ColumnFloat
	FilledPrice              postgres.ColumnFloat
	FilledAt                 postgres.ColumnTimestampz
	CreatedAt                postgres.ColumnTimestampz
	ModifiedAt               postgres.ColumnTimestampz
	Notes                    postgres.ColumnString

	AllColumns     postgres.ColumnList
	MutableColumns postgres.ColumnList
}

type TradeOrderTable struct {
	tradeOrderTable

	EXCLUDED tradeOrderTable
}

// AS creates new TradeOrderTable with assigned alias
func (a TradeOrderTable) AS(alias string) *TradeOrderTable {
	return newTradeOrderTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new TradeOrderTable with assigned schema name
func (a TradeOrderTable) FromSchema(schemaName string) *TradeOrderTable {
	return newTradeOrderTable(schemaName, a.TableName(), a.Alias())
}

func newTradeOrderTable(schemaName, tableName, alias string) *TradeOrderTable {
	return &TradeOrderTable{
		tradeOrderTable: newTradeOrderTableImpl(schemaName, tableName, alias),
		EXCLUDED:        newTradeOrderTableImpl("", "excluded", ""),
	}
}

func newTradeOrderTableImpl(schemaName, tableName, alias string) tradeOrderTable {
	var (
		TradeOrderIDColumn             = postgres.StringColumn("trade_order_id")
		ProviderIDColumn               = postgres.StringColumn("provider_id")
		TickerIDColumn                 = postgres.StringColumn("ticker_id")
		SideColumn                     = postgres.StringColumn("side")
		RequestedAmountInDollarsColumn = postgres.FloatColumn("requested_amount_in_dollars")
		StatusColumn                   = postgres.StringColumn("status")
		FilledQuantityColumn           = postgres.FloatColumn("filled_quantity")
		FilledPriceColumn              = postgres.FloatColumn("filled_price")
		FilledAtColumn                 = postgres.TimestampzColumn("filled_at")
		CreatedAtColumn                = postgres.TimestampzColumn("created_at")
		ModifiedAtColumn               = postgres.TimestampzColumn("modified_at")
		NotesColumn                    = postgres.StringColumn("notes")
		allColumns                     = postgres.ColumnList{TradeOrderIDColumn, ProviderIDColumn, TickerIDColumn, SideColumn, RequestedAmountInDollarsColumn, StatusColumn, FilledQuantityColumn, FilledPriceColumn, FilledAtColumn, CreatedAtColumn, ModifiedAtColumn, NotesColumn}
		mutableColumns                 = postgres.ColumnList{ProviderIDColumn, TickerIDColumn, SideColumn, RequestedAmountInDollarsColumn, StatusColumn, FilledQuantityColumn, FilledPriceColumn, FilledAtColumn, CreatedAtColumn, ModifiedAtColumn, NotesColumn}
	)

	return tradeOrderTable{
		Table: postgres.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		TradeOrderID:             TradeOrderIDColumn,
		ProviderID:               ProviderIDColumn,
		TickerID:                 TickerIDColumn,
		Side:                     SideColumn,
		RequestedAmountInDollars: RequestedAmountInDollarsColumn,
		Status:                   StatusColumn,
		FilledQuantity:           FilledQuantityColumn,
		FilledPrice:              FilledPriceColumn,
		FilledAt:                 FilledAtColumn,
		CreatedAt:                CreatedAtColumn,
		ModifiedAt:               ModifiedAtColumn,
		Notes:                    NotesColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}
