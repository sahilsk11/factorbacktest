//
// Code generated by go-jet DO NOT EDIT.
//
// WARNING: Changes to this file may cause incorrect behavior
// and will be lost if the code is regenerated
//

package view

import (
	"github.com/go-jet/jet/v2/postgres"
)

var LatestRebalanceHoldings = newLatestRebalanceHoldingsTable("public", "latest_rebalance_holdings", "")

type latestRebalanceHoldingsTable struct {
	postgres.Table

	//Columns
	InvestmentHoldingsID        postgres.ColumnString
	InvestmentID                postgres.ColumnString
	Symbol                      postgres.ColumnString
	Quantity                    postgres.ColumnFloat
	PriceAtRebalance            postgres.ColumnFloat
	AmountAtRebalance           postgres.ColumnFloat
	CreatedAt                   postgres.ColumnTimestampz
	TickerID                    postgres.ColumnString
	InvestmentHoldingsVersionID postgres.ColumnString
	RebalancerRunID             postgres.ColumnString

	AllColumns     postgres.ColumnList
	MutableColumns postgres.ColumnList
}

type LatestRebalanceHoldingsTable struct {
	latestRebalanceHoldingsTable

	EXCLUDED latestRebalanceHoldingsTable
}

// AS creates new LatestRebalanceHoldingsTable with assigned alias
func (a LatestRebalanceHoldingsTable) AS(alias string) *LatestRebalanceHoldingsTable {
	return newLatestRebalanceHoldingsTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new LatestRebalanceHoldingsTable with assigned schema name
func (a LatestRebalanceHoldingsTable) FromSchema(schemaName string) *LatestRebalanceHoldingsTable {
	return newLatestRebalanceHoldingsTable(schemaName, a.TableName(), a.Alias())
}

func newLatestRebalanceHoldingsTable(schemaName, tableName, alias string) *LatestRebalanceHoldingsTable {
	return &LatestRebalanceHoldingsTable{
		latestRebalanceHoldingsTable: newLatestRebalanceHoldingsTableImpl(schemaName, tableName, alias),
		EXCLUDED:                     newLatestRebalanceHoldingsTableImpl("", "excluded", ""),
	}
}

func newLatestRebalanceHoldingsTableImpl(schemaName, tableName, alias string) latestRebalanceHoldingsTable {
	var (
		InvestmentHoldingsIDColumn        = postgres.StringColumn("investment_holdings_id")
		InvestmentIDColumn                = postgres.StringColumn("investment_id")
		SymbolColumn                      = postgres.StringColumn("symbol")
		QuantityColumn                    = postgres.FloatColumn("quantity")
		PriceAtRebalanceColumn            = postgres.FloatColumn("price_at_rebalance")
		AmountAtRebalanceColumn           = postgres.FloatColumn("amount_at_rebalance")
		CreatedAtColumn                   = postgres.TimestampzColumn("created_at")
		TickerIDColumn                    = postgres.StringColumn("ticker_id")
		InvestmentHoldingsVersionIDColumn = postgres.StringColumn("investment_holdings_version_id")
		RebalancerRunIDColumn             = postgres.StringColumn("rebalancer_run_id")
		allColumns                        = postgres.ColumnList{InvestmentHoldingsIDColumn, InvestmentIDColumn, SymbolColumn, QuantityColumn, PriceAtRebalanceColumn, AmountAtRebalanceColumn, CreatedAtColumn, TickerIDColumn, InvestmentHoldingsVersionIDColumn, RebalancerRunIDColumn}
		mutableColumns                    = postgres.ColumnList{InvestmentHoldingsIDColumn, InvestmentIDColumn, SymbolColumn, QuantityColumn, PriceAtRebalanceColumn, AmountAtRebalanceColumn, CreatedAtColumn, TickerIDColumn, InvestmentHoldingsVersionIDColumn, RebalancerRunIDColumn}
	)

	return latestRebalanceHoldingsTable{
		Table: postgres.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		InvestmentHoldingsID:        InvestmentHoldingsIDColumn,
		InvestmentID:                InvestmentIDColumn,
		Symbol:                      SymbolColumn,
		Quantity:                    QuantityColumn,
		PriceAtRebalance:            PriceAtRebalanceColumn,
		AmountAtRebalance:           AmountAtRebalanceColumn,
		CreatedAt:                   CreatedAtColumn,
		TickerID:                    TickerIDColumn,
		InvestmentHoldingsVersionID: InvestmentHoldingsVersionIDColumn,
		RebalancerRunID:             RebalancerRunIDColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}
