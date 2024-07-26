package main

import (
	"context"
	"factorbacktest/cmd"
	"factorbacktest/internal/logger"
	"log"

	_ "github.com/lib/pq"
)

func main() {
	apiHandler, err := cmd.InitializeDependencies()
	if err != nil {
		log.Fatal(err)
	}

	lg := logger.New()
	ctx := context.WithValue(context.Background(), logger.ContextKey, lg)

	err = apiHandler.StartApi(ctx, 3009)
	if err != nil {
		log.Fatal(err)
	}
}
