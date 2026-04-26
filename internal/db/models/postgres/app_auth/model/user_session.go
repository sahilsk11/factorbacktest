//
// HAND-WRITTEN to match the shape go-jet would generate for the
// app_auth.user_session table introduced in migration 000053. If
// `make db-models` is ever extended to generate from the app_auth
// schema, this file should be the target output.
//

package model

import (
	"time"

	"github.com/google/uuid"
)

type UserSession struct {
	ID            string `sql:"primary_key"`
	UserAccountID uuid.UUID
	CreatedAt     time.Time
	ExpiresAt     time.Time
	LastSeenAt    time.Time
	IP            *string
	UserAgent     *string
}
