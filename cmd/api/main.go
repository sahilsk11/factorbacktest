package main

import (
	"context"
	"factorbacktest/cmd"
	"factorbacktest/internal/logger"
	"factorbacktest/internal/util"
	"log"
	"time"

	_ "github.com/lib/pq"
)

func main() {
	secrets, err := util.LoadSecrets()
	if err != nil {
		log.Fatalf("failed to load secrets: %v", err)
	}

	secrets.Port = 3009

	apiHandler, err := cmd.InitializeDependencies(*secrets, nil)
	if err != nil {
		log.Fatal(err)
	}

	lg := logger.New()
	ctx := context.WithValue(context.Background(), logger.ContextKey, lg)

	// Run a daily cleanup of expired session rows so app_auth.user_session
	// doesn't accumulate forever. Best-effort: a transient DB error during
	// a sweep is logged and the next tick tries again.
	if apiHandler.AuthService != nil {
		go apiHandler.AuthService.RunSessionCleanup(ctx, 24*time.Hour)
	}

	err = apiHandler.StartApi(ctx)
	if err != nil {
		log.Fatal(err)
	}
}
