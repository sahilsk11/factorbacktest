package main

import (
	"factorbacktest/internal/util"
	"factorbacktest/tools/seeds"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	db, err := util.NewTestDb()
	if err != nil {
		logger.Fatal("failed to connect to db", zap.Error(err), zap.String("db_notes", "util.NewTestDb"))
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		logger.Fatal("failed to begin transaction", zap.Error(err), zap.String("db_notes", "db.Begin"))
	}

	hammer := seeds.NewHammer(tx)
	hammer.Commit = true
	hammer.CsvPath = "tools/seeds/sample_prices_2020.csv"

	userID := uuid.NewString()
	if err := hammer.SeedAll(userID); err != nil {
		tx.Rollback()
		logger.Fatal("failed to seed", zap.String("userID", userID), zap.Error(err), zap.String("db_notes", "hammer.SeedAll"))
	}

	logger.Info("successfully seeded database", zap.String("userID", userID))
}
