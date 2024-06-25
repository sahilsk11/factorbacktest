//
// Code generated by go-jet DO NOT EDIT.
//
// WARNING: Changes to this file may cause incorrect behavior
// and will be lost if the code is regenerated
//

package model

import (
	"github.com/google/uuid"
	"time"
)

type APIRequest struct {
	RequestID    uuid.UUID `sql:"primary_key"`
	UserID       *uuid.UUID
	IPAddress    *string
	Method       string
	Route        string
	RequestBody  *string
	StartTs      time.Time
	DurationMs   *int64
	StatusCode   *int32
	ResponseBody *string
}
