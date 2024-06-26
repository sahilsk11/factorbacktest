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

var LatencyTracking = newLatencyTrackingTable("public", "latency_tracking", "")

type latencyTrackingTable struct {
	postgres.Table

	//Columns
	LatencyTrackingID postgres.ColumnString
	ProcessingTimes   postgres.ColumnString
	TotalProcessingMs postgres.ColumnInteger
	RequestID         postgres.ColumnString

	AllColumns     postgres.ColumnList
	MutableColumns postgres.ColumnList
}

type LatencyTrackingTable struct {
	latencyTrackingTable

	EXCLUDED latencyTrackingTable
}

// AS creates new LatencyTrackingTable with assigned alias
func (a LatencyTrackingTable) AS(alias string) *LatencyTrackingTable {
	return newLatencyTrackingTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new LatencyTrackingTable with assigned schema name
func (a LatencyTrackingTable) FromSchema(schemaName string) *LatencyTrackingTable {
	return newLatencyTrackingTable(schemaName, a.TableName(), a.Alias())
}

func newLatencyTrackingTable(schemaName, tableName, alias string) *LatencyTrackingTable {
	return &LatencyTrackingTable{
		latencyTrackingTable: newLatencyTrackingTableImpl(schemaName, tableName, alias),
		EXCLUDED:             newLatencyTrackingTableImpl("", "excluded", ""),
	}
}

func newLatencyTrackingTableImpl(schemaName, tableName, alias string) latencyTrackingTable {
	var (
		LatencyTrackingIDColumn = postgres.StringColumn("latency_tracking_id")
		ProcessingTimesColumn   = postgres.StringColumn("processing_times")
		TotalProcessingMsColumn = postgres.IntegerColumn("total_processing_ms")
		RequestIDColumn         = postgres.StringColumn("request_id")
		allColumns              = postgres.ColumnList{LatencyTrackingIDColumn, ProcessingTimesColumn, TotalProcessingMsColumn, RequestIDColumn}
		mutableColumns          = postgres.ColumnList{ProcessingTimesColumn, TotalProcessingMsColumn, RequestIDColumn}
	)

	return latencyTrackingTable{
		Table: postgres.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		LatencyTrackingID: LatencyTrackingIDColumn,
		ProcessingTimes:   ProcessingTimesColumn,
		TotalProcessingMs: TotalProcessingMsColumn,
		RequestID:         RequestIDColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}
