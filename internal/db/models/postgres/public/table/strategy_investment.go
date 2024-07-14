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

var StrategyInvestment = newStrategyInvestmentTable("public", "strategy_investment", "")

type strategyInvestmentTable struct {
	postgres.Table

	//Columns
	StrategyInvestmentID postgres.ColumnString
	AmountDollars        postgres.ColumnInteger
	StartDate            postgres.ColumnDate
	SavedStragyID        postgres.ColumnString
	UserAccountID        postgres.ColumnString
	CreatedAt            postgres.ColumnTimestampz
	ModifiedAt           postgres.ColumnTimestampz
	EndDate              postgres.ColumnDate

	AllColumns     postgres.ColumnList
	MutableColumns postgres.ColumnList
}

type StrategyInvestmentTable struct {
	strategyInvestmentTable

	EXCLUDED strategyInvestmentTable
}

// AS creates new StrategyInvestmentTable with assigned alias
func (a StrategyInvestmentTable) AS(alias string) *StrategyInvestmentTable {
	return newStrategyInvestmentTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new StrategyInvestmentTable with assigned schema name
func (a StrategyInvestmentTable) FromSchema(schemaName string) *StrategyInvestmentTable {
	return newStrategyInvestmentTable(schemaName, a.TableName(), a.Alias())
}

func newStrategyInvestmentTable(schemaName, tableName, alias string) *StrategyInvestmentTable {
	return &StrategyInvestmentTable{
		strategyInvestmentTable: newStrategyInvestmentTableImpl(schemaName, tableName, alias),
		EXCLUDED:                newStrategyInvestmentTableImpl("", "excluded", ""),
	}
}

func newStrategyInvestmentTableImpl(schemaName, tableName, alias string) strategyInvestmentTable {
	var (
		StrategyInvestmentIDColumn = postgres.StringColumn("strategy_investment_id")
		AmountDollarsColumn        = postgres.IntegerColumn("amount_dollars")
		StartDateColumn            = postgres.DateColumn("start_date")
		SavedStragyIDColumn        = postgres.StringColumn("saved_stragy_id")
		UserAccountIDColumn        = postgres.StringColumn("user_account_id")
		CreatedAtColumn            = postgres.TimestampzColumn("created_at")
		ModifiedAtColumn           = postgres.TimestampzColumn("modified_at")
		EndDateColumn              = postgres.DateColumn("end_date")
		allColumns                 = postgres.ColumnList{StrategyInvestmentIDColumn, AmountDollarsColumn, StartDateColumn, SavedStragyIDColumn, UserAccountIDColumn, CreatedAtColumn, ModifiedAtColumn, EndDateColumn}
		mutableColumns             = postgres.ColumnList{AmountDollarsColumn, StartDateColumn, SavedStragyIDColumn, UserAccountIDColumn, CreatedAtColumn, ModifiedAtColumn, EndDateColumn}
	)

	return strategyInvestmentTable{
		Table: postgres.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		StrategyInvestmentID: StrategyInvestmentIDColumn,
		AmountDollars:        AmountDollarsColumn,
		StartDate:            StartDateColumn,
		SavedStragyID:        SavedStragyIDColumn,
		UserAccountID:        UserAccountIDColumn,
		CreatedAt:            CreatedAtColumn,
		ModifiedAt:           ModifiedAtColumn,
		EndDate:              EndDateColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}