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

var UserAccount = newUserAccountTable("public", "user_account", "")

type userAccountTable struct {
	postgres.Table

	//Columns
	UserAccountID postgres.ColumnString
	FirstName     postgres.ColumnString
	LastName      postgres.ColumnString
	Email         postgres.ColumnString
	CreatedAt     postgres.ColumnTimestampz
	UpdatedAt     postgres.ColumnTimestampz
	Provider      postgres.ColumnString
	ProviderID    postgres.ColumnString
	PhoneNumber   postgres.ColumnString

	AllColumns     postgres.ColumnList
	MutableColumns postgres.ColumnList
}

type UserAccountTable struct {
	userAccountTable

	EXCLUDED userAccountTable
}

// AS creates new UserAccountTable with assigned alias
func (a UserAccountTable) AS(alias string) *UserAccountTable {
	return newUserAccountTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new UserAccountTable with assigned schema name
func (a UserAccountTable) FromSchema(schemaName string) *UserAccountTable {
	return newUserAccountTable(schemaName, a.TableName(), a.Alias())
}

func newUserAccountTable(schemaName, tableName, alias string) *UserAccountTable {
	return &UserAccountTable{
		userAccountTable: newUserAccountTableImpl(schemaName, tableName, alias),
		EXCLUDED:         newUserAccountTableImpl("", "excluded", ""),
	}
}

func newUserAccountTableImpl(schemaName, tableName, alias string) userAccountTable {
	var (
		UserAccountIDColumn = postgres.StringColumn("user_account_id")
		FirstNameColumn     = postgres.StringColumn("first_name")
		LastNameColumn      = postgres.StringColumn("last_name")
		EmailColumn         = postgres.StringColumn("email")
		CreatedAtColumn     = postgres.TimestampzColumn("created_at")
		UpdatedAtColumn     = postgres.TimestampzColumn("updated_at")
		ProviderColumn      = postgres.StringColumn("provider")
		ProviderIDColumn    = postgres.StringColumn("provider_id")
		PhoneNumberColumn   = postgres.StringColumn("phone_number")
		allColumns          = postgres.ColumnList{UserAccountIDColumn, FirstNameColumn, LastNameColumn, EmailColumn, CreatedAtColumn, UpdatedAtColumn, ProviderColumn, ProviderIDColumn, PhoneNumberColumn}
		mutableColumns      = postgres.ColumnList{FirstNameColumn, LastNameColumn, EmailColumn, CreatedAtColumn, UpdatedAtColumn, ProviderColumn, ProviderIDColumn, PhoneNumberColumn}
	)

	return userAccountTable{
		Table: postgres.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		UserAccountID: UserAccountIDColumn,
		FirstName:     FirstNameColumn,
		LastName:      LastNameColumn,
		Email:         EmailColumn,
		CreatedAt:     CreatedAtColumn,
		UpdatedAt:     UpdatedAtColumn,
		Provider:      ProviderColumn,
		ProviderID:    ProviderIDColumn,
		PhoneNumber:   PhoneNumberColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}
