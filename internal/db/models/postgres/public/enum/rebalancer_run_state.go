//
// Code generated by go-jet DO NOT EDIT.
//
// WARNING: Changes to this file may cause incorrect behavior
// and will be lost if the code is regenerated
//

package enum

import "github.com/go-jet/jet/v2/postgres"

var RebalancerRunState = &struct {
	Completed postgres.StringExpression
	Pending   postgres.StringExpression
	Error     postgres.StringExpression
}{
	Completed: postgres.NewEnumValue("COMPLETED"),
	Pending:   postgres.NewEnumValue("PENDING"),
	Error:     postgres.NewEnumValue("ERROR"),
}