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

var PublishedStrategy = newPublishedStrategyTable("public", "published_strategy", "")

type publishedStrategyTable struct {
	postgres.Table

	//Columns
	PublishedStrategyID postgres.ColumnString
	CreatedAt           postgres.ColumnTimestampz
	ModifiedAt          postgres.ColumnTimestampz
	DeletedAt           postgres.ColumnTimestampz
	SavedStrategyID     postgres.ColumnString

	AllColumns     postgres.ColumnList
	MutableColumns postgres.ColumnList
}

type PublishedStrategyTable struct {
	publishedStrategyTable

	EXCLUDED publishedStrategyTable
}

// AS creates new PublishedStrategyTable with assigned alias
func (a PublishedStrategyTable) AS(alias string) *PublishedStrategyTable {
	return newPublishedStrategyTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new PublishedStrategyTable with assigned schema name
func (a PublishedStrategyTable) FromSchema(schemaName string) *PublishedStrategyTable {
	return newPublishedStrategyTable(schemaName, a.TableName(), a.Alias())
}

func newPublishedStrategyTable(schemaName, tableName, alias string) *PublishedStrategyTable {
	return &PublishedStrategyTable{
		publishedStrategyTable: newPublishedStrategyTableImpl(schemaName, tableName, alias),
		EXCLUDED:               newPublishedStrategyTableImpl("", "excluded", ""),
	}
}

func newPublishedStrategyTableImpl(schemaName, tableName, alias string) publishedStrategyTable {
	var (
		PublishedStrategyIDColumn = postgres.StringColumn("published_strategy_id")
		CreatedAtColumn           = postgres.TimestampzColumn("created_at")
		ModifiedAtColumn          = postgres.TimestampzColumn("modified_at")
		DeletedAtColumn           = postgres.TimestampzColumn("deleted_at")
		SavedStrategyIDColumn     = postgres.StringColumn("saved_strategy_id")
		allColumns                = postgres.ColumnList{PublishedStrategyIDColumn, CreatedAtColumn, ModifiedAtColumn, DeletedAtColumn, SavedStrategyIDColumn}
		mutableColumns            = postgres.ColumnList{CreatedAtColumn, ModifiedAtColumn, DeletedAtColumn, SavedStrategyIDColumn}
	)

	return publishedStrategyTable{
		Table: postgres.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		PublishedStrategyID: PublishedStrategyIDColumn,
		CreatedAt:           CreatedAtColumn,
		ModifiedAt:          ModifiedAtColumn,
		DeletedAt:           DeletedAtColumn,
		SavedStrategyID:     SavedStrategyIDColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}
