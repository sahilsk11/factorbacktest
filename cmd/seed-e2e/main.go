package main

import (
	"log"

	"factorbacktest/internal/util"
	"factorbacktest/tools/seeds"

	"github.com/google/uuid"
)

func main() {
	db, err := util.NewTestDb()
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("failed to begin transaction: %v", err)
	}

	hammer := seeds.NewHammer(tx)
	hammer.Commit = true
	hammer.CsvPath = "tools/seeds/sample_prices_2020.csv"

	userID := uuid.NewString()
	if err := hammer.SeedAll(userID); err != nil {
		tx.Rollback()
		log.Fatalf("failed to seed: %v", err)
	}

	log.Printf("Successfully seeded database with userID=%s", userID)
}
