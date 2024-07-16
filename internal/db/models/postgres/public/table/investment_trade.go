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
	InvestmentTradeID postgres.ColumnString
	TickerID          postgres.ColumnString
	AmountInDollars   postgres.ColumnFloat
	Side              postgres.ColumnString
	CreatedAt         postgres.ColumnTimestampz
	InvestmentID      postgres.ColumnString
	RebalancerRunID   postgres.ColumnString

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
		InvestmentTradeIDColumn = postgres.StringColumn("investment_trade_id")
		TickerIDColumn          = postgres.StringColumn("ticker_id")
		AmountInDollarsColumn   = postgres.FloatColumn("amount_in_dollars")
		SideColumn              = postgres.StringColumn("side")
		CreatedAtColumn         = postgres.TimestampzColumn("created_at")
		InvestmentIDColumn      = postgres.StringColumn("investment_id")
		RebalancerRunIDColumn   = postgres.StringColumn("rebalancer_run_id")
		allColumns              = postgres.ColumnList{InvestmentTradeIDColumn, TickerIDColumn, AmountInDollarsColumn, SideColumn, CreatedAtColumn, InvestmentIDColumn, RebalancerRunIDColumn}
		mutableColumns          = postgres.ColumnList{TickerIDColumn, AmountInDollarsColumn, SideColumn, CreatedAtColumn, InvestmentIDColumn, RebalancerRunIDColumn}
	)

	return investmentTradeTable{
		Table: postgres.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		InvestmentTradeID: InvestmentTradeIDColumn,
		TickerID:          TickerIDColumn,
		AmountInDollars:   AmountInDollarsColumn,
		Side:              SideColumn,
		CreatedAt:         CreatedAtColumn,
		InvestmentID:      InvestmentIDColumn,
		RebalancerRunID:   RebalancerRunIDColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}