package repository

import (
	"alpha/internal/db/models/postgres/public/model"
	. "alpha/internal/db/models/postgres/public/table"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"
)

type ContactRepository interface {
	// use db here bc I don't really want to use
	// application logic tx for this. also should
	// be committed instantly
	Add(db qrm.Executable, c model.ContactMessage) error
}

type ContactRepositoryHandler struct{}

func (h ContactRepositoryHandler) Add(db qrm.Executable, c model.ContactMessage) error {
	c.MessageID = uuid.New()
	c.CreatedAt = time.Now().UTC()
	query := ContactMessage.
		INSERT(ContactMessage.MutableColumns).
		MODEL(c)

	_, err := query.Exec(db)
	if err != nil {
		return fmt.Errorf("failed to insert contact message: %w", err)
	}

	return nil
}
