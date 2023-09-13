package repository

import (
	"database/sql"
	"fmt"
)

type UsageStats struct {
	UniqueUsers      int `json:"uniqueUsers"`
	BacktestsRun     int `json:"backtests"`
	StrategiesTested int `json:"strategies"`
}

func GetUsageStats(tx *sql.DB) (*UsageStats, error) {
	query := `select
	(select count(distinct user_id) from api_request) as "distinct_users",
	(select count(*) from user_strategy) as "num_backtests_run",
	(select count(distinct factor_expression_hash) from user_strategy) as "distinct_strategies";`

	row := tx.QueryRow(query)

	out := UsageStats{}

	err := row.Scan(&out.UniqueUsers, &out.BacktestsRun, &out.StrategiesTested)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage stats: %w", err)
	}

	return &out, nil
}
