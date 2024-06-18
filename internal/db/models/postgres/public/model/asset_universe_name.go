//
// Code generated by go-jet DO NOT EDIT.
//
// WARNING: Changes to this file may cause incorrect behavior
// and will be lost if the code is regenerated
//

package model

import "errors"

type AssetUniverseName string

const (
	AssetUniverseName_SpyTop80 AssetUniverseName = "SPY_TOP_80"
	AssetUniverseName_All      AssetUniverseName = "ALL"
)

func (e *AssetUniverseName) Scan(value interface{}) error {
	var enumValue string
	switch val := value.(type) {
	case string:
		enumValue = val
	case []byte:
		enumValue = string(val)
	default:
		return errors.New("jet: Invalid scan value for AllTypesEnum enum. Enum value has to be of type string or []byte")
	}

	switch enumValue {
	case "SPY_TOP_80":
		*e = AssetUniverseName_SpyTop80
	case "ALL":
		*e = AssetUniverseName_All
	default:
		return errors.New("jet: Invalid scan value '" + enumValue + "' for AssetUniverseName enum")
	}

	return nil
}

func (e AssetUniverseName) String() string {
	return string(e)
}
