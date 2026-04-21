package main

import (
	"context"
	"factorbacktest/cmd"
	"factorbacktest/internal/logger"
	"factorbacktest/internal/util"
	"log"

	_ "github.com/lib/pq"
)

func main() {
	secrets, err := util.LoadSecrets()
	if err != nil {
		log.Fatalf("failed to load secrets: %v", err)
	}

	apiHandler, err := cmd.InitializeDependencies(*secrets)
	if err != nil {
		log.Fatal(err)
	}

	lg := logger.New()
	ctx := context.WithValue(context.Background(), logger.ContextKey, lg)

	err = apiHandler.StartApi(ctx)
	if err != nil {
		log.Fatal(err)
	}
}
