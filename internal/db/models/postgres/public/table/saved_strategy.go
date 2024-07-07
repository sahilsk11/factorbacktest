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

var SavedStrategy = newSavedStrategyTable("public", "saved_strategy", "")

type savedStrategyTable struct {
	postgres.Table

	//Columns
	SavedStragyID     postgres.ColumnString
	StrategyName      postgres.ColumnString
	FactorExpression  postgres.ColumnString
	BacktestStart     postgres.ColumnDate
	BacktestEnd       postgres.ColumnDate
	RebalanceInterval postgres.ColumnString
	NumAssets         postgres.ColumnInteger
	AssetUniverse     postgres.ColumnString
	Bookmarked        postgres.ColumnBool
	UserAccountID     postgres.ColumnString
	CreatedAt         postgres.ColumnTimestampz
	ModifiedAt        postgres.ColumnTimestampz

	AllColumns     postgres.ColumnList
	MutableColumns postgres.ColumnList
}

type SavedStrategyTable struct {
	savedStrategyTable

	EXCLUDED savedStrategyTable
}

// AS creates new SavedStrategyTable with assigned alias
func (a SavedStrategyTable) AS(alias string) *SavedStrategyTable {
	return newSavedStrategyTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new SavedStrategyTable with assigned schema name
func (a SavedStrategyTable) FromSchema(schemaName string) *SavedStrategyTable {
	return newSavedStrategyTable(schemaName, a.TableName(), a.Alias())
}

func newSavedStrategyTable(schemaName, tableName, alias string) *SavedStrategyTable {
	return &SavedStrategyTable{
		savedStrategyTable: newSavedStrategyTableImpl(schemaName, tableName, alias),
		EXCLUDED:           newSavedStrategyTableImpl("", "excluded", ""),
	}
}

func newSavedStrategyTableImpl(schemaName, tableName, alias string) savedStrategyTable {
	var (
		SavedStragyIDColumn     = postgres.StringColumn("saved_stragy_id")
		StrategyNameColumn      = postgres.StringColumn("strategy_name")
		FactorExpressionColumn  = postgres.StringColumn("factor_expression")
		BacktestStartColumn     = postgres.DateColumn("backtest_start")
		BacktestEndColumn       = postgres.DateColumn("backtest_end")
		RebalanceIntervalColumn = postgres.StringColumn("rebalance_interval")
		NumAssetsColumn         = postgres.IntegerColumn("num_assets")
		AssetUniverseColumn     = postgres.StringColumn("asset_universe")
		BookmarkedColumn        = postgres.BoolColumn("bookmarked")
		UserAccountIDColumn     = postgres.StringColumn("user_account_id")
		CreatedAtColumn         = postgres.TimestampzColumn("created_at")
		ModifiedAtColumn        = postgres.TimestampzColumn("modified_at")
		allColumns              = postgres.ColumnList{SavedStragyIDColumn, StrategyNameColumn, FactorExpressionColumn, BacktestStartColumn, BacktestEndColumn, RebalanceIntervalColumn, NumAssetsColumn, AssetUniverseColumn, BookmarkedColumn, UserAccountIDColumn, CreatedAtColumn, ModifiedAtColumn}
		mutableColumns          = postgres.ColumnList{StrategyNameColumn, FactorExpressionColumn, BacktestStartColumn, BacktestEndColumn, RebalanceIntervalColumn, NumAssetsColumn, AssetUniverseColumn, BookmarkedColumn, UserAccountIDColumn, CreatedAtColumn, ModifiedAtColumn}
	)

	return savedStrategyTable{
		Table: postgres.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		SavedStragyID:     SavedStragyIDColumn,
		StrategyName:      StrategyNameColumn,
		FactorExpression:  FactorExpressionColumn,
		BacktestStart:     BacktestStartColumn,
		BacktestEnd:       BacktestEndColumn,
		RebalanceInterval: RebalanceIntervalColumn,
		NumAssets:         NumAssetsColumn,
		AssetUniverse:     AssetUniverseColumn,
		Bookmarked:        BookmarkedColumn,
		UserAccountID:     UserAccountIDColumn,
		CreatedAt:         CreatedAtColumn,
		ModifiedAt:        ModifiedAtColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}
