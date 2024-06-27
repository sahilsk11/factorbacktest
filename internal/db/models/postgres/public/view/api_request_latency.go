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

var APIRequestLatency = newAPIRequestLatencyTable("public", "api_request_latency", "")

type aPIRequestLatencyTable struct {
	postgres.Table

	//Columns
	RequestID         postgres.ColumnString
	Route             postgres.ColumnString
	StartTs           postgres.ColumnTimestampz
	TotalProcessingMs postgres.ColumnInteger
	ProcessingTimes   postgres.ColumnString
	Version           postgres.ColumnString

	AllColumns     postgres.ColumnList
	MutableColumns postgres.ColumnList
}

type APIRequestLatencyTable struct {
	aPIRequestLatencyTable

	EXCLUDED aPIRequestLatencyTable
}

// AS creates new APIRequestLatencyTable with assigned alias
func (a APIRequestLatencyTable) AS(alias string) *APIRequestLatencyTable {
	return newAPIRequestLatencyTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new APIRequestLatencyTable with assigned schema name
func (a APIRequestLatencyTable) FromSchema(schemaName string) *APIRequestLatencyTable {
	return newAPIRequestLatencyTable(schemaName, a.TableName(), a.Alias())
}

func newAPIRequestLatencyTable(schemaName, tableName, alias string) *APIRequestLatencyTable {
	return &APIRequestLatencyTable{
		aPIRequestLatencyTable: newAPIRequestLatencyTableImpl(schemaName, tableName, alias),
		EXCLUDED:               newAPIRequestLatencyTableImpl("", "excluded", ""),
	}
}

func newAPIRequestLatencyTableImpl(schemaName, tableName, alias string) aPIRequestLatencyTable {
	var (
		RequestIDColumn         = postgres.StringColumn("request_id")
		RouteColumn             = postgres.StringColumn("route")
		StartTsColumn           = postgres.TimestampzColumn("start_ts")
		TotalProcessingMsColumn = postgres.IntegerColumn("total_processing_ms")
		ProcessingTimesColumn   = postgres.StringColumn("processing_times")
		VersionColumn           = postgres.StringColumn("version")
		allColumns              = postgres.ColumnList{RequestIDColumn, RouteColumn, StartTsColumn, TotalProcessingMsColumn, ProcessingTimesColumn, VersionColumn}
		mutableColumns          = postgres.ColumnList{RequestIDColumn, RouteColumn, StartTsColumn, TotalProcessingMsColumn, ProcessingTimesColumn, VersionColumn}
	)

	return aPIRequestLatencyTable{
		Table: postgres.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		RequestID:         RequestIDColumn,
		Route:             RouteColumn,
		StartTs:           StartTsColumn,
		TotalProcessingMs: TotalProcessingMsColumn,
		ProcessingTimes:   ProcessingTimesColumn,
		Version:           VersionColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}
