package main

import (
	"context"
	"factorbacktest/api"
	"factorbacktest/cmd"
	"factorbacktest/internal/domain"
	"log"

	_ "github.com/lib/pq"
)

func main() {
	handler, err := cmd.InitializeDependencies()
	if err != nil {
		log.Fatal(err)
	}

	profile, endProfile := domain.NewProfile()
	defer endProfile()
	ctx := context.WithValue(context.Background(), domain.ContextProfileKey, profile)

	// err = handler.TradingService.Re()
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// updateOrders(handler)

	err = handler.InvestmentService.Reconcile(ctx)
	if err != nil {
		log.Fatal(err)
	}

}

func updateOrders(handler *api.ApiHandler) {
	err := handler.TradingService.UpdateAllPendingOrders()
	if err != nil {
		log.Fatal(err)
	}
}
