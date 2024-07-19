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

var RebalancerRun = newRebalancerRunTable("public", "rebalancer_run", "")

type rebalancerRunTable struct {
	postgres.Table

	//Columns
	RebalancerRunID         postgres.ColumnString
	Date                    postgres.ColumnDate
	CreatedAt               postgres.ColumnTimestampz
	RebalancerRunType       postgres.ColumnString
	RebalancerRunState      postgres.ColumnString
	ModifiedAt              postgres.ColumnTimestampz
	NumInvestmentsAttempted postgres.ColumnInteger

	AllColumns     postgres.ColumnList
	MutableColumns postgres.ColumnList
}

type RebalancerRunTable struct {
	rebalancerRunTable

	EXCLUDED rebalancerRunTable
}

// AS creates new RebalancerRunTable with assigned alias
func (a RebalancerRunTable) AS(alias string) *RebalancerRunTable {
	return newRebalancerRunTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new RebalancerRunTable with assigned schema name
func (a RebalancerRunTable) FromSchema(schemaName string) *RebalancerRunTable {
	return newRebalancerRunTable(schemaName, a.TableName(), a.Alias())
}

func newRebalancerRunTable(schemaName, tableName, alias string) *RebalancerRunTable {
	return &RebalancerRunTable{
		rebalancerRunTable: newRebalancerRunTableImpl(schemaName, tableName, alias),
		EXCLUDED:           newRebalancerRunTableImpl("", "excluded", ""),
	}
}

func newRebalancerRunTableImpl(schemaName, tableName, alias string) rebalancerRunTable {
	var (
		RebalancerRunIDColumn         = postgres.StringColumn("rebalancer_run_id")
		DateColumn                    = postgres.DateColumn("date")
		CreatedAtColumn               = postgres.TimestampzColumn("created_at")
		RebalancerRunTypeColumn       = postgres.StringColumn("rebalancer_run_type")
		RebalancerRunStateColumn      = postgres.StringColumn("rebalancer_run_state")
		ModifiedAtColumn              = postgres.TimestampzColumn("modified_at")
		NumInvestmentsAttemptedColumn = postgres.IntegerColumn("num_investments_attempted")
		allColumns                    = postgres.ColumnList{RebalancerRunIDColumn, DateColumn, CreatedAtColumn, RebalancerRunTypeColumn, RebalancerRunStateColumn, ModifiedAtColumn, NumInvestmentsAttemptedColumn}
		mutableColumns                = postgres.ColumnList{DateColumn, CreatedAtColumn, RebalancerRunTypeColumn, RebalancerRunStateColumn, ModifiedAtColumn, NumInvestmentsAttemptedColumn}
	)

	return rebalancerRunTable{
		Table: postgres.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		RebalancerRunID:         RebalancerRunIDColumn,
		Date:                    DateColumn,
		CreatedAt:               CreatedAtColumn,
		RebalancerRunType:       RebalancerRunTypeColumn,
		RebalancerRunState:      RebalancerRunStateColumn,
		ModifiedAt:              ModifiedAtColumn,
		NumInvestmentsAttempted: NumInvestmentsAttemptedColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}
