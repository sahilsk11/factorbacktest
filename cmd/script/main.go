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

	err = handler.InvestmentService.Rebalance(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// updateOrders(handler)

	// err = handler.InvestmentService.Reconcile(ctx, uuid.MustParse("b50cba85-45c1-4182-8172-b5a1166fea3d"))
	// if err != nil {
	// 	log.Fatal(err)
	// }

}

func updateOrders(handler *api.ApiHandler) {
	err := handler.TradingService.UpdateAllPendingOrders()
	if err != nil {
		log.Fatal(err)
	}
}
