package main

import (
	"context"
	"factorbacktest/api"
	"factorbacktest/cmd"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/logger"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "factorbacktest",
	Short: "A tool for running various factor backtests.",
	Long:  `A tool for running various factor backtests, with options for reconciliation and updating orders.`,
}

func init() {
	rootCmd.AddCommand(reconcileCmd)
	rootCmd.AddCommand(updateOrdersCmd)
}

var reconcileCmd = &cobra.Command{
	Use:   "reconcile",
	Short: "Reconcile the investment data",
	Run: func(c *cobra.Command, args []string) {
		handler, err := cmd.InitializeDependencies()
		if err != nil {
			log.Fatal(err)
		}

		profile, endProfile := domain.NewProfile()
		defer endProfile()
		ctx := context.WithValue(context.Background(), domain.ContextProfileKey, profile)

		l := logger.New()
		ctx = context.WithValue(ctx, logger.ContextKey, l)

		logger.Info("reconciling")

		err = handler.InvestmentService.Reconcile(ctx)
		if err != nil {
			l.Error(err.Error())
			log.Fatal(err)
		}
	},
}

var rebalanceCmd = &cobra.Command{
	Use:   "rebalance",
	Short: "Rebalance the investment data",
	Run: func(c *cobra.Command, args []string) {
		handler, err := cmd.InitializeDependencies()
		if err != nil {
			log.Fatal(err)
		}

		profile, endProfile := domain.NewProfile()
		defer endProfile()
		ctx := context.WithValue(context.Background(), domain.ContextProfileKey, profile)
		lg := logger.New()
		ctx = context.WithValue(ctx, logger.ContextKey, lg)

		logger.Info("rebalancing")

		err = handler.InvestmentService.Rebalance(ctx)
		if err != nil {
			lg.Fatal(err.Error())
		}
	},
}

var updateOrdersCmd = &cobra.Command{
	Use:   "update-orders",
	Short: "Update all pending orders",
	Run: func(c *cobra.Command, args []string) {
		handler, err := cmd.InitializeDependencies()
		if err != nil {
			log.Fatal(err)
		}
		logger.Info("updating orders")

		updateOrders(handler)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func main() {
	Execute()
}

func updateOrders(handler *api.ApiHandler) {
	err := handler.TradingService.UpdateAllPendingOrders()
	if err != nil {
		log.Fatal(err)
	}
}
