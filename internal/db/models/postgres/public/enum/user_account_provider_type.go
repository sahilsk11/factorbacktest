//
// Code generated by go-jet DO NOT EDIT.
//
// WARNING: Changes to this file may cause incorrect behavior
// and will be lost if the code is regenerated
//

package enum

import "github.com/go-jet/jet/v2/postgres"

var UserAccountProviderType = &struct {
	Supabase postgres.StringExpression
	Google   postgres.StringExpression
	Manual   postgres.StringExpression
}{
	Supabase: postgres.NewEnumValue("SUPABASE"),
	Google:   postgres.NewEnumValue("GOOGLE"),
	Manual:   postgres.NewEnumValue("MANUAL"),
}
