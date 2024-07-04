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

var APIRequest = newAPIRequestTable("public", "api_request", "")

type aPIRequestTable struct {
	postgres.Table

	//Columns
	RequestID     postgres.ColumnString
	UserID        postgres.ColumnString
	IPAddress     postgres.ColumnString
	Method        postgres.ColumnString
	Route         postgres.ColumnString
	RequestBody   postgres.ColumnString
	StartTs       postgres.ColumnTimestampz
	DurationMs    postgres.ColumnInteger
	StatusCode    postgres.ColumnInteger
	ResponseBody  postgres.ColumnString
	Version       postgres.ColumnString
	UserAccountID postgres.ColumnString

	AllColumns     postgres.ColumnList
	MutableColumns postgres.ColumnList
}

type APIRequestTable struct {
	aPIRequestTable

	EXCLUDED aPIRequestTable
}

// AS creates new APIRequestTable with assigned alias
func (a APIRequestTable) AS(alias string) *APIRequestTable {
	return newAPIRequestTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new APIRequestTable with assigned schema name
func (a APIRequestTable) FromSchema(schemaName string) *APIRequestTable {
	return newAPIRequestTable(schemaName, a.TableName(), a.Alias())
}

func newAPIRequestTable(schemaName, tableName, alias string) *APIRequestTable {
	return &APIRequestTable{
		aPIRequestTable: newAPIRequestTableImpl(schemaName, tableName, alias),
		EXCLUDED:        newAPIRequestTableImpl("", "excluded", ""),
	}
}

func newAPIRequestTableImpl(schemaName, tableName, alias string) aPIRequestTable {
	var (
		RequestIDColumn     = postgres.StringColumn("request_id")
		UserIDColumn        = postgres.StringColumn("user_id")
		IPAddressColumn     = postgres.StringColumn("ip_address")
		MethodColumn        = postgres.StringColumn("method")
		RouteColumn         = postgres.StringColumn("route")
		RequestBodyColumn   = postgres.StringColumn("request_body")
		StartTsColumn       = postgres.TimestampzColumn("start_ts")
		DurationMsColumn    = postgres.IntegerColumn("duration_ms")
		StatusCodeColumn    = postgres.IntegerColumn("status_code")
		ResponseBodyColumn  = postgres.StringColumn("response_body")
		VersionColumn       = postgres.StringColumn("version")
		UserAccountIDColumn = postgres.StringColumn("user_account_id")
		allColumns          = postgres.ColumnList{RequestIDColumn, UserIDColumn, IPAddressColumn, MethodColumn, RouteColumn, RequestBodyColumn, StartTsColumn, DurationMsColumn, StatusCodeColumn, ResponseBodyColumn, VersionColumn, UserAccountIDColumn}
		mutableColumns      = postgres.ColumnList{UserIDColumn, IPAddressColumn, MethodColumn, RouteColumn, RequestBodyColumn, StartTsColumn, DurationMsColumn, StatusCodeColumn, ResponseBodyColumn, VersionColumn, UserAccountIDColumn}
	)

	return aPIRequestTable{
		Table: postgres.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		RequestID:     RequestIDColumn,
		UserID:        UserIDColumn,
		IPAddress:     IPAddressColumn,
		Method:        MethodColumn,
		Route:         RouteColumn,
		RequestBody:   RequestBodyColumn,
		StartTs:       StartTsColumn,
		DurationMs:    DurationMsColumn,
		StatusCode:    StatusCodeColumn,
		ResponseBody:  ResponseBodyColumn,
		Version:       VersionColumn,
		UserAccountID: UserAccountIDColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}
