package main

import (
	"context"
	"factorbacktest/cmd"
	"factorbacktest/internal/logger"
	"factorbacktest/internal/util"
	"log"
	"os"
	"strconv"

	_ "github.com/lib/pq"
)

func main() {
	secrets, err := util.LoadSecrets()
	if err != nil {
		log.Fatalf("failed to load secrets: %v", err)
	}

	secrets.Port = 3009
	if port := os.Getenv("PORT"); port != "" {
		parsed, err := strconv.Atoi(port)
		if err != nil || parsed <= 0 {
			log.Fatalf("invalid PORT %q: %v", port, err)
		}
		secrets.Port = parsed
	}

	apiHandler, err := cmd.InitializeDependencies(*secrets, nil)
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
