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

var AssetUniverseSize = newAssetUniverseSizeTable("public", "asset_universe_size", "")

type assetUniverseSizeTable struct {
	postgres.Table

	//Columns
	DisplayName       postgres.ColumnString
	AssetUniverseName postgres.ColumnString
	NumAssets         postgres.ColumnInteger

	AllColumns     postgres.ColumnList
	MutableColumns postgres.ColumnList
}

type AssetUniverseSizeTable struct {
	assetUniverseSizeTable

	EXCLUDED assetUniverseSizeTable
}

// AS creates new AssetUniverseSizeTable with assigned alias
func (a AssetUniverseSizeTable) AS(alias string) *AssetUniverseSizeTable {
	return newAssetUniverseSizeTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new AssetUniverseSizeTable with assigned schema name
func (a AssetUniverseSizeTable) FromSchema(schemaName string) *AssetUniverseSizeTable {
	return newAssetUniverseSizeTable(schemaName, a.TableName(), a.Alias())
}

func newAssetUniverseSizeTable(schemaName, tableName, alias string) *AssetUniverseSizeTable {
	return &AssetUniverseSizeTable{
		assetUniverseSizeTable: newAssetUniverseSizeTableImpl(schemaName, tableName, alias),
		EXCLUDED:               newAssetUniverseSizeTableImpl("", "excluded", ""),
	}
}

func newAssetUniverseSizeTableImpl(schemaName, tableName, alias string) assetUniverseSizeTable {
	var (
		DisplayNameColumn       = postgres.StringColumn("display_name")
		AssetUniverseNameColumn = postgres.StringColumn("asset_universe_name")
		NumAssetsColumn         = postgres.IntegerColumn("num_assets")
		allColumns              = postgres.ColumnList{DisplayNameColumn, AssetUniverseNameColumn, NumAssetsColumn}
		mutableColumns          = postgres.ColumnList{DisplayNameColumn, AssetUniverseNameColumn, NumAssetsColumn}
	)

	return assetUniverseSizeTable{
		Table: postgres.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		DisplayName:       DisplayNameColumn,
		AssetUniverseName: AssetUniverseNameColumn,
		NumAssets:         NumAssetsColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}
