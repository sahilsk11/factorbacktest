package repository

import (
	"database/sql"
	"fmt"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"

	"github.com/go-jet/jet/v2/qrm"
)

const (
	AdminMutationKindLedgerAdjust   = "ledger_adjust"
	AdminMutationKindBrokerDustSell = "broker_dust_sell"
)

type AdminMutationAuditRepository interface {
	Add(tx *sql.Tx, row model.AdminMutationAudit) (*model.AdminMutationAudit, error)
}

type adminMutationAuditRepositoryHandler struct {
	Db *sql.DB
}

func NewAdminMutationAuditRepository(db *sql.DB) AdminMutationAuditRepository {
	return adminMutationAuditRepositoryHandler{Db: db}
}

func (h adminMutationAuditRepositoryHandler) Add(tx *sql.Tx, row model.AdminMutationAudit) (*model.AdminMutationAudit, error) {
	if row.CreatedAt.IsZero() {
		row.CreatedAt = time.Now().UTC()
	}
	if row.Payload == "" {
		row.Payload = "{}"
	}

	query := table.AdminMutationAudit.
		INSERT(table.AdminMutationAudit.MutableColumns).
		MODEL(row).
		RETURNING(table.AdminMutationAudit.AllColumns)

	var db qrm.Queryable = h.Db
	if tx != nil {
		db = tx
	}

	out := model.AdminMutationAudit{}
	if err := query.Query(db, &out); err != nil {
		return nil, fmt.Errorf("insert admin_mutation_audit: %w", err)
	}
	return &out, nil
}
