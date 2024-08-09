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

var RebalancePrice = newRebalancePriceTable("public", "rebalance_price", "")

type rebalancePriceTable struct {
	postgres.Table

	//Columns
	RebalancePriceID postgres.ColumnString
	TickerID         postgres.ColumnString
	Price            postgres.ColumnFloat
	RebalancerRunID  postgres.ColumnString
	CreatedAt        postgres.ColumnTimestampz

	AllColumns     postgres.ColumnList
	MutableColumns postgres.ColumnList
}

type RebalancePriceTable struct {
	rebalancePriceTable

	EXCLUDED rebalancePriceTable
}

// AS creates new RebalancePriceTable with assigned alias
func (a RebalancePriceTable) AS(alias string) *RebalancePriceTable {
	return newRebalancePriceTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new RebalancePriceTable with assigned schema name
func (a RebalancePriceTable) FromSchema(schemaName string) *RebalancePriceTable {
	return newRebalancePriceTable(schemaName, a.TableName(), a.Alias())
}

func newRebalancePriceTable(schemaName, tableName, alias string) *RebalancePriceTable {
	return &RebalancePriceTable{
		rebalancePriceTable: newRebalancePriceTableImpl(schemaName, tableName, alias),
		EXCLUDED:            newRebalancePriceTableImpl("", "excluded", ""),
	}
}

func newRebalancePriceTableImpl(schemaName, tableName, alias string) rebalancePriceTable {
	var (
		RebalancePriceIDColumn = postgres.StringColumn("rebalance_price_id")
		TickerIDColumn         = postgres.StringColumn("ticker_id")
		PriceColumn            = postgres.FloatColumn("price")
		RebalancerRunIDColumn  = postgres.StringColumn("rebalancer_run_id")
		CreatedAtColumn        = postgres.TimestampzColumn("created_at")
		allColumns             = postgres.ColumnList{RebalancePriceIDColumn, TickerIDColumn, PriceColumn, RebalancerRunIDColumn, CreatedAtColumn}
		mutableColumns         = postgres.ColumnList{TickerIDColumn, PriceColumn, RebalancerRunIDColumn, CreatedAtColumn}
	)

	return rebalancePriceTable{
		Table: postgres.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		RebalancePriceID: RebalancePriceIDColumn,
		TickerID:         TickerIDColumn,
		Price:            PriceColumn,
		RebalancerRunID:  RebalancerRunIDColumn,
		CreatedAt:        CreatedAtColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}
