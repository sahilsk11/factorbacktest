//
// HAND-WRITTEN to match the shape go-jet would generate for the
// app_auth.user_session table. See ../model/user_session.go for the
// matching struct.
//

package table

import (
	"github.com/go-jet/jet/v2/postgres"
)

var UserSession = newUserSessionTable("app_auth", "user_session", "")

type userSessionTable struct {
	postgres.Table

	ID            postgres.ColumnString
	UserAccountID postgres.ColumnString
	CreatedAt     postgres.ColumnTimestampz
	ExpiresAt     postgres.ColumnTimestampz
	LastSeenAt    postgres.ColumnTimestampz
	IP            postgres.ColumnString
	UserAgent     postgres.ColumnString

	AllColumns     postgres.ColumnList
	MutableColumns postgres.ColumnList
}

type UserSessionTable struct {
	userSessionTable

	EXCLUDED userSessionTable
}

func (a UserSessionTable) AS(alias string) *UserSessionTable {
	return newUserSessionTable(a.SchemaName(), a.TableName(), alias)
}

func (a UserSessionTable) FromSchema(schemaName string) *UserSessionTable {
	return newUserSessionTable(schemaName, a.TableName(), a.Alias())
}

func newUserSessionTable(schemaName, tableName, alias string) *UserSessionTable {
	return &UserSessionTable{
		userSessionTable: newUserSessionTableImpl(schemaName, tableName, alias),
		EXCLUDED:         newUserSessionTableImpl("", "excluded", ""),
	}
}

func newUserSessionTableImpl(schemaName, tableName, alias string) userSessionTable {
	var (
		IDColumn            = postgres.StringColumn("id")
		UserAccountIDColumn = postgres.StringColumn("user_account_id")
		CreatedAtColumn     = postgres.TimestampzColumn("created_at")
		ExpiresAtColumn     = postgres.TimestampzColumn("expires_at")
		LastSeenAtColumn    = postgres.TimestampzColumn("last_seen_at")
		IPColumn            = postgres.StringColumn("ip")
		UserAgentColumn     = postgres.StringColumn("user_agent")
		allColumns          = postgres.ColumnList{IDColumn, UserAccountIDColumn, CreatedAtColumn, ExpiresAtColumn, LastSeenAtColumn, IPColumn, UserAgentColumn}
		mutableColumns      = postgres.ColumnList{UserAccountIDColumn, CreatedAtColumn, ExpiresAtColumn, LastSeenAtColumn, IPColumn, UserAgentColumn}
	)

	return userSessionTable{
		Table:         postgres.NewTable(schemaName, tableName, alias, allColumns...),
		ID:            IDColumn,
		UserAccountID: UserAccountIDColumn,
		CreatedAt:     CreatedAtColumn,
		ExpiresAt:     ExpiresAtColumn,
		LastSeenAt:    LastSeenAtColumn,
		IP:            IPColumn,
		UserAgent:     UserAgentColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}
