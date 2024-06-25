//
// Code generated by go-jet DO NOT EDIT.
//
// WARNING: Changes to this file may cause incorrect behavior
// and will be lost if the code is regenerated
//

package model

import "errors"

type AssetFundamentalGranularity string

const (
	AssetFundamentalGranularity_Quarterly AssetFundamentalGranularity = "QUARTERLY"
	AssetFundamentalGranularity_Annual    AssetFundamentalGranularity = "ANNUAL"
)

func (e *AssetFundamentalGranularity) Scan(value interface{}) error {
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
	case "QUARTERLY":
		*e = AssetFundamentalGranularity_Quarterly
	case "ANNUAL":
		*e = AssetFundamentalGranularity_Annual
	default:
		return errors.New("jet: Invalid scan value '" + enumValue + "' for AssetFundamentalGranularity enum")
	}

	return nil
}

func (e AssetFundamentalGranularity) String() string {
	return string(e)
}