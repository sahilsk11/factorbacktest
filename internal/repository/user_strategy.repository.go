package repository

import (
	"alpha/internal/db/models/postgres/public/model"
	. "alpha/internal/db/models/postgres/public/table"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"
)

type UserStrategyRepository interface {
	// use db here bc I don't really want to use
	// application logic tx for this. also should
	// be committed instantly
	Add(db qrm.Executable, us model.UserStrategy) error
}

type UserStrategyRepositoryHandler struct{}

func (h UniverseRepositoryHandler) Add(db qrm.Executable, us model.UserStrategy) error {
	us.UserStrategyID = uuid.New()
	us.CreatedAt = time.Now().UTC()
	query := UserStrategy.
		INSERT(UserStrategy.MutableColumns).
		MODEL(us)

	_, err := query.Exec(db)
	if err != nil {
		return fmt.Errorf("failed to insert user strategy: %w", err)
	}

	return nil
}
