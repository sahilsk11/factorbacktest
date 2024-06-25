package main

import (
	"alpha/api"
	"alpha/internal"
	"alpha/internal/app"
	"alpha/internal/repository"
	"database/sql"
	"log"

	_ "github.com/lib/pq"
)

func New() (*sql.DB, error) {
	connStr := "user=postgres password=dVucP6jSZqx7yyPOsz1v host=alpha.cuutadkicrvi.us-east-2.rds.amazonaws.com port=5432 dbname=postgres"
	dbConn, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	return dbConn, nil
}

func main() {
	dbConn, err := New()
	if err != nil {
		log.Fatal(err)
	}
	apiHandler := api.ApiHandler{
		BenchmarkHandler: internal.BenchmarkHandler{
			PriceRepository: repository.AdjustedPriceRepositoryHandler{},
		},
		BacktestHandler: app.BacktestHandler{
			PriceRepository: repository.AdjustedPriceRepositoryHandler{},
			FactorMetricsHandler: internal.FactorMetricsHandler{
				AdjustedPriceRepository:     repository.AdjustedPriceRepositoryHandler{},
				AssetFundamentalsRepository: repository.AssetFundamentalsRepositoryHandler{},
			},
			UniverseRepository: repository.UniverseRepositoryHandler{},
		},
		Db: dbConn,
	}
	err = apiHandler.StartApi(3009)
	if err != nil {
		log.Fatal(err)
	}
}
