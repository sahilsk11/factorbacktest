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

var InvestmentTrade = newInvestmentTradeTable("public", "investment_trade", "")

type investmentTradeTable struct {
	postgres.Table

	//Columns
	InvestmentTradeID     postgres.ColumnString
	TickerID              postgres.ColumnString
	Side                  postgres.ColumnString
	CreatedAt             postgres.ColumnTimestampz
	Quantity              postgres.ColumnFloat
	TradeOrderID          postgres.ColumnString
	ModifiedAt            postgres.ColumnTimestampz
	InvestmentRebalanceID postgres.ColumnString

	AllColumns     postgres.ColumnList
	MutableColumns postgres.ColumnList
}

type InvestmentTradeTable struct {
	investmentTradeTable

	EXCLUDED investmentTradeTable
}

// AS creates new InvestmentTradeTable with assigned alias
func (a InvestmentTradeTable) AS(alias string) *InvestmentTradeTable {
	return newInvestmentTradeTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new InvestmentTradeTable with assigned schema name
func (a InvestmentTradeTable) FromSchema(schemaName string) *InvestmentTradeTable {
	return newInvestmentTradeTable(schemaName, a.TableName(), a.Alias())
}

func newInvestmentTradeTable(schemaName, tableName, alias string) *InvestmentTradeTable {
	return &InvestmentTradeTable{
		investmentTradeTable: newInvestmentTradeTableImpl(schemaName, tableName, alias),
		EXCLUDED:             newInvestmentTradeTableImpl("", "excluded", ""),
	}
}

func newInvestmentTradeTableImpl(schemaName, tableName, alias string) investmentTradeTable {
	var (
		InvestmentTradeIDColumn     = postgres.StringColumn("investment_trade_id")
		TickerIDColumn              = postgres.StringColumn("ticker_id")
		SideColumn                  = postgres.StringColumn("side")
		CreatedAtColumn             = postgres.TimestampzColumn("created_at")
		QuantityColumn              = postgres.FloatColumn("quantity")
		TradeOrderIDColumn          = postgres.StringColumn("trade_order_id")
		ModifiedAtColumn            = postgres.TimestampzColumn("modified_at")
		InvestmentRebalanceIDColumn = postgres.StringColumn("investment_rebalance_id")
		allColumns                  = postgres.ColumnList{InvestmentTradeIDColumn, TickerIDColumn, SideColumn, CreatedAtColumn, QuantityColumn, TradeOrderIDColumn, ModifiedAtColumn, InvestmentRebalanceIDColumn}
		mutableColumns              = postgres.ColumnList{TickerIDColumn, SideColumn, CreatedAtColumn, QuantityColumn, TradeOrderIDColumn, ModifiedAtColumn, InvestmentRebalanceIDColumn}
	)

	return investmentTradeTable{
		Table: postgres.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		InvestmentTradeID:     InvestmentTradeIDColumn,
		TickerID:              TickerIDColumn,
		Side:                  SideColumn,
		CreatedAt:             CreatedAtColumn,
		Quantity:              QuantityColumn,
		TradeOrderID:          TradeOrderIDColumn,
		ModifiedAt:            ModifiedAtColumn,
		InvestmentRebalanceID: InvestmentRebalanceIDColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}
