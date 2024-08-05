package main

import (
	"context"
	"factorbacktest/api"
	"factorbacktest/cmd"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/logger"
	"factorbacktest/internal/util"
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
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
	rootCmd.AddCommand(rebalanceCmd)
	rootCmd.AddCommand(updateOrdersCmd)
	rootCmd.AddCommand(updatePublishedStrategyStats)
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

		l.Info("reconciling")

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

		lg.Info("rebalancing")

		err = handler.InvestmentService.Rebalance(ctx)
		if err != nil {
			lg.Fatal(err.Error())
		}
	},
}

var updateOrdersCmd = &cobra.Command{
	Use:   "update",
	Short: "Update all pending orders",
	Run: func(c *cobra.Command, args []string) {

		handler, err := cmd.InitializeDependencies()
		if err != nil {
			log.Fatal(err)
		}
		log := logger.New()
		log.Info("updating orders")

		updateOrders(handler)

	},
}

var updatePublishedStrategyStats = &cobra.Command{
	Use:   "updateStats",
	Short: "Update all pending orders",
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

		metrics, err := handler.InvestmentService.CalculateMetrics(ctx, uuid.MustParse("00186fdc-93a0-4686-a0d1-848d532bf12a"))
		if err != nil {
			lg.Error(err)
		}

		util.Pprint(metrics)

		metrics, err = handler.InvestmentService.CalculateMetrics(ctx, uuid.MustParse("5531ef32-ae2d-4e10-88b6-15eee887289b"))
		if err != nil {
			lg.Error(err)
		}

		util.Pprint(metrics)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func main() {
	os.Setenv("ALPHA_ENV", "dev")
	Execute()
}

func updateOrders(handler *api.ApiHandler) {
	err := handler.TradingService.UpdateAllPendingOrders()
	if err != nil {
		log.Fatal(err)
	}
}
