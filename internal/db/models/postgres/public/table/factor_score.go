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

var FactorScore = newFactorScoreTable("public", "factor_score", "")

type factorScoreTable struct {
	postgres.Table

	//Columns
	FactorScoreID        postgres.ColumnString
	TickerID             postgres.ColumnString
	FactorExpressionHash postgres.ColumnString
	Date                 postgres.ColumnDate
	Score                postgres.ColumnFloat
	CreatedAt            postgres.ColumnTimestampz
	UpdatedAt            postgres.ColumnTimestampz
	Error                postgres.ColumnString

	AllColumns     postgres.ColumnList
	MutableColumns postgres.ColumnList
}

type FactorScoreTable struct {
	factorScoreTable

	EXCLUDED factorScoreTable
}

// AS creates new FactorScoreTable with assigned alias
func (a FactorScoreTable) AS(alias string) *FactorScoreTable {
	return newFactorScoreTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new FactorScoreTable with assigned schema name
func (a FactorScoreTable) FromSchema(schemaName string) *FactorScoreTable {
	return newFactorScoreTable(schemaName, a.TableName(), a.Alias())
}

func newFactorScoreTable(schemaName, tableName, alias string) *FactorScoreTable {
	return &FactorScoreTable{
		factorScoreTable: newFactorScoreTableImpl(schemaName, tableName, alias),
		EXCLUDED:         newFactorScoreTableImpl("", "excluded", ""),
	}
}

func newFactorScoreTableImpl(schemaName, tableName, alias string) factorScoreTable {
	var (
		FactorScoreIDColumn        = postgres.StringColumn("factor_score_id")
		TickerIDColumn             = postgres.StringColumn("ticker_id")
		FactorExpressionHashColumn = postgres.StringColumn("factor_expression_hash")
		DateColumn                 = postgres.DateColumn("date")
		ScoreColumn                = postgres.FloatColumn("score")
		CreatedAtColumn            = postgres.TimestampzColumn("created_at")
		UpdatedAtColumn            = postgres.TimestampzColumn("updated_at")
		ErrorColumn                = postgres.StringColumn("error")
		allColumns                 = postgres.ColumnList{FactorScoreIDColumn, TickerIDColumn, FactorExpressionHashColumn, DateColumn, ScoreColumn, CreatedAtColumn, UpdatedAtColumn, ErrorColumn}
		mutableColumns             = postgres.ColumnList{TickerIDColumn, FactorExpressionHashColumn, DateColumn, ScoreColumn, CreatedAtColumn, UpdatedAtColumn, ErrorColumn}
	)

	return factorScoreTable{
		Table: postgres.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		FactorScoreID:        FactorScoreIDColumn,
		TickerID:             TickerIDColumn,
		FactorExpressionHash: FactorExpressionHashColumn,
		Date:                 DateColumn,
		Score:                ScoreColumn,
		CreatedAt:            CreatedAtColumn,
		UpdatedAt:            UpdatedAtColumn,
		Error:                ErrorColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}
