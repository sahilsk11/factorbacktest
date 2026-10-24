package service

import (
	"context"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	mock_repository "factorbacktest/internal/repository/mocks"
	"sort"
	"testing"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func dec(s string) decimal.Decimal {
	return decimal.RequireFromString(s)
}

func testDate(daysFromAnchor int) time.Time {
	anchor := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	return anchor.AddDate(0, 0, daysFromAnchor)
}

func priceAt(symbol string, price string, date time.Time) domain.AssetPrice {
	return domain.AssetPrice{
		Symbol: symbol,
		Price:  dec(price),
		Date:   date,
	}
}

func issueByTypeSymbol(t *testing.T, issues []ReconIssue, issueType, symbol string) ReconIssue {
	t.Helper()
	for _, issue := range issues {
		if issue.Type == issueType && issue.Symbol == symbol {
			return issue
		}
	}
	t.Fatalf("missing issue type=%s symbol=%s in %+v", issueType, symbol, issues)
	return ReconIssue{}
}

func requireQtyFields(t *testing.T, issue ReconIssue, ledger, alpaca, diff float64) {
	t.Helper()
	require.NotNil(t, issue.LedgerQty)
	require.NotNil(t, issue.AlpacaQty)
	require.NotNil(t, issue.Diff)
	require.InDelta(t, ledger, *issue.LedgerQty, 1e-9)
	require.InDelta(t, alpaca, *issue.AlpacaQty, 1e-9)
	require.InDelta(t, diff, *issue.Diff, 1e-9)
}

func TestCompareAggregatePortfolios(t *testing.T) {
	now := testDate(0)
	staleBefore := testDate(-5)
	fresh := now

	tests := []struct {
		name       string
		in         aggregateReconcileInput
		wantStatus string
		wantCount  int
		assert     func(t *testing.T, issues []ReconIssue)
	}{
		{
			name: "matching cash and positions is OK",
			in: aggregateReconcileInput{
				LedgerCash:  dec("100"),
				AlpacaCash:  dec("100"),
				LedgerQty:   map[string]decimal.Decimal{"AAPL": dec("1.5")},
				AlpacaQty:   map[string]decimal.Decimal{"AAPL": dec("1.5")},
				Prices:      map[string]domain.AssetPrice{"AAPL": priceAt("AAPL", "150", fresh)},
				StaleBefore: staleBefore,
			},
			wantStatus: ReconStatusOK,
			wantCount:  0,
		},
		{
			name: "PHANTOM_POSITION when ledger holds a symbol Alpaca does not",
			in: aggregateReconcileInput{
				LedgerCash:  decimal.Zero,
				AlpacaCash:  decimal.Zero,
				LedgerQty:   map[string]decimal.Decimal{"VST": dec("0.20")},
				AlpacaQty:   map[string]decimal.Decimal{},
				Prices:      map[string]domain.AssetPrice{"VST": priceAt("VST", "100", fresh)},
				StaleBefore: staleBefore,
			},
			wantStatus: ReconStatusDustOnly,
			wantCount:  1,
			assert: func(t *testing.T, issues []ReconIssue) {
				issue := issues[0]
				require.Equal(t, ReconIssuePhantomPosition, issue.Type)
				require.Equal(t, ReconSeverityDust, issue.Severity)
				require.Equal(t, "VST", issue.Symbol)
				requireQtyFields(t, issue, 0.20, 0, -0.20)
				require.NotNil(t, issue.DollarValue)
				require.InDelta(t, -20, *issue.DollarValue, 1e-9)
			},
		},
		{
			name: "UNTRACKED_POSITION when Alpaca holds a symbol ledger does not",
			in: aggregateReconcileInput{
				LedgerCash:  decimal.Zero,
				AlpacaCash:  decimal.Zero,
				LedgerQty:   map[string]decimal.Decimal{},
				AlpacaQty:   map[string]decimal.Decimal{"MSFT": dec("2")},
				Prices:      map[string]domain.AssetPrice{"MSFT": priceAt("MSFT", "400", fresh)},
				StaleBefore: staleBefore,
			},
			wantStatus: ReconStatusDrift,
			wantCount:  1,
			assert: func(t *testing.T, issues []ReconIssue) {
				issue := issues[0]
				require.Equal(t, ReconIssueUntrackedPosition, issue.Type)
				require.Equal(t, ReconSeverityDrift, issue.Severity)
				require.Equal(t, "MSFT", issue.Symbol)
				requireQtyFields(t, issue, 0, 2, 2)
				require.NotNil(t, issue.DollarValue)
				require.InDelta(t, 800, *issue.DollarValue, 1e-9)
			},
		},
		{
			name: "QUANTITY_MISMATCH replaces the 2-share threshold",
			in: aggregateReconcileInput{
				LedgerCash:  decimal.Zero,
				AlpacaCash:  decimal.Zero,
				LedgerQty:   map[string]decimal.Decimal{"INTC": dec("1.197")},
				AlpacaQty:   map[string]decimal.Decimal{"INTC": dec("0.764")},
				Prices:      map[string]domain.AssetPrice{"INTC": priceAt("INTC", "20", fresh)},
				StaleBefore: staleBefore,
			},
			wantStatus: ReconStatusDustOnly,
			wantCount:  1,
			assert: func(t *testing.T, issues []ReconIssue) {
				issue := issues[0]
				require.Equal(t, ReconIssueQuantityMismatch, issue.Type)
				require.Equal(t, ReconSeverityDust, issue.Severity)
				require.Equal(t, "INTC", issue.Symbol)
				requireQtyFields(t, issue, 1.197, 0.764, -0.433)
				require.NotNil(t, issue.DollarValue)
				require.InDelta(t, -8.66, *issue.DollarValue, 1e-9)
			},
		},
		{
			name: "quantity epsilon equal to 1e-6 is not a mismatch",
			in: aggregateReconcileInput{
				LedgerCash:  decimal.Zero,
				AlpacaCash:  decimal.Zero,
				LedgerQty:   map[string]decimal.Decimal{"AAPL": dec("1.000001")},
				AlpacaQty:   map[string]decimal.Decimal{"AAPL": dec("1")},
				Prices:      map[string]domain.AssetPrice{"AAPL": priceAt("AAPL", "150", fresh)},
				StaleBefore: staleBefore,
			},
			wantStatus: ReconStatusOK,
			wantCount:  0,
		},
		{
			name: "quantity just above 1e-6 is QUANTITY_MISMATCH",
			in: aggregateReconcileInput{
				LedgerCash:  decimal.Zero,
				AlpacaCash:  decimal.Zero,
				LedgerQty:   map[string]decimal.Decimal{"AAPL": dec("1.000002")},
				AlpacaQty:   map[string]decimal.Decimal{"AAPL": dec("1")},
				Prices:      map[string]domain.AssetPrice{"AAPL": priceAt("AAPL", "150", fresh)},
				StaleBefore: staleBefore,
			},
			wantStatus: ReconStatusDustOnly,
			wantCount:  1,
			assert: func(t *testing.T, issues []ReconIssue) {
				require.Equal(t, ReconIssueQuantityMismatch, issues[0].Type)
				require.Equal(t, "AAPL", issues[0].Symbol)
			},
		},
		{
			name: "qty at or below epsilon on one side is treated as zero",
			in: aggregateReconcileInput{
				LedgerCash:  decimal.Zero,
				AlpacaCash:  decimal.Zero,
				LedgerQty:   map[string]decimal.Decimal{"VST": dec("0.000001")},
				AlpacaQty:   map[string]decimal.Decimal{},
				Prices:      map[string]domain.AssetPrice{"VST": priceAt("VST", "100", fresh)},
				StaleBefore: staleBefore,
			},
			wantStatus: ReconStatusOK,
			wantCount:  0,
		},
		{
			name: "CASH_MISMATCH above 0.50 tolerance",
			in: aggregateReconcileInput{
				LedgerCash:  dec("100.00"),
				AlpacaCash:  dec("99.40"),
				LedgerQty:   map[string]decimal.Decimal{},
				AlpacaQty:   map[string]decimal.Decimal{},
				StaleBefore: staleBefore,
			},
			wantStatus: ReconStatusDustOnly,
			wantCount:  1,
			assert: func(t *testing.T, issues []ReconIssue) {
				issue := issues[0]
				require.Equal(t, ReconIssueCashMismatch, issue.Type)
				require.Equal(t, ReconSeverityDust, issue.Severity)
				require.Equal(t, ReconCashSymbol, issue.Symbol)
				requireQtyFields(t, issue, 100, 99.40, -0.60)
				require.NotNil(t, issue.DollarValue)
				require.InDelta(t, -0.60, *issue.DollarValue, 1e-9)
			},
		},
		{
			name: "cash diff equal to 0.50 tolerance is not an issue",
			in: aggregateReconcileInput{
				LedgerCash: dec("100.00"),
				AlpacaCash: dec("99.50"),
			},
			wantStatus: ReconStatusOK,
			wantCount:  0,
		},
		{
			name: "cash just above 0.50 tolerance is CASH_MISMATCH",
			in: aggregateReconcileInput{
				LedgerCash: dec("100.00"),
				AlpacaCash: dec("99.49"),
			},
			wantStatus: ReconStatusDustOnly,
			wantCount:  1,
			assert: func(t *testing.T, issues []ReconIssue) {
				require.Equal(t, ReconIssueCashMismatch, issues[0].Type)
			},
		},
		{
			name: "cash mismatch at 50 dollars is DRIFT",
			in: aggregateReconcileInput{
				LedgerCash: dec("100"),
				AlpacaCash: dec("50"),
			},
			wantStatus: ReconStatusDrift,
			wantCount:  1,
			assert: func(t *testing.T, issues []ReconIssue) {
				require.Equal(t, ReconIssueCashMismatch, issues[0].Type)
				require.Equal(t, ReconSeverityDrift, issues[0].Severity)
			},
		},
		{
			name: "dollar value strictly below 50 is DUST",
			in: aggregateReconcileInput{
				LedgerQty:   map[string]decimal.Decimal{"GEV": dec("0.49")},
				AlpacaQty:   map[string]decimal.Decimal{},
				Prices:      map[string]domain.AssetPrice{"GEV": priceAt("GEV", "100", fresh)},
				StaleBefore: staleBefore,
			},
			wantStatus: ReconStatusDustOnly,
			wantCount:  1,
			assert: func(t *testing.T, issues []ReconIssue) {
				require.Equal(t, ReconSeverityDust, issues[0].Severity)
				require.Equal(t, ReconIssuePhantomPosition, issues[0].Type)
			},
		},
		{
			name: "dollar value of 50 is DRIFT",
			in: aggregateReconcileInput{
				LedgerQty:   map[string]decimal.Decimal{"GEV": dec("0.50")},
				AlpacaQty:   map[string]decimal.Decimal{},
				Prices:      map[string]domain.AssetPrice{"GEV": priceAt("GEV", "100", fresh)},
				StaleBefore: staleBefore,
			},
			wantStatus: ReconStatusDrift,
			wantCount:  1,
			assert: func(t *testing.T, issues []ReconIssue) {
				require.Equal(t, ReconSeverityDrift, issues[0].Severity)
			},
		},
		{
			name: "missing price on a qty mismatch is DRIFT",
			in: aggregateReconcileInput{
				LedgerQty:   map[string]decimal.Decimal{"PCG": dec("1")},
				AlpacaQty:   map[string]decimal.Decimal{},
				Prices:      map[string]domain.AssetPrice{},
				StaleBefore: staleBefore,
			},
			wantStatus: ReconStatusDrift,
			wantCount:  2,
			assert: func(t *testing.T, issues []ReconIssue) {
				phantom := issueByTypeSymbol(t, issues, ReconIssuePhantomPosition, "PCG")
				require.Equal(t, ReconSeverityDrift, phantom.Severity)
				require.Nil(t, phantom.DollarValue)
				stale := issueByTypeSymbol(t, issues, ReconIssueStalePrice, "PCG")
				require.Equal(t, ReconSeverityDrift, stale.Severity)
				require.Contains(t, stale.Message, "no adjusted price found")
			},
		},
		{
			name: "STALE_PRICE when latest adjusted price is older than cutoff",
			in: aggregateReconcileInput{
				LedgerQty: map[string]decimal.Decimal{"NSC": dec("1")},
				AlpacaQty: map[string]decimal.Decimal{"NSC": dec("1")},
				Prices: map[string]domain.AssetPrice{
					"NSC": priceAt("NSC", "200", testDate(-6)),
				},
				StaleBefore: staleBefore,
			},
			wantStatus: ReconStatusDrift,
			wantCount:  1,
			assert: func(t *testing.T, issues []ReconIssue) {
				issue := issues[0]
				require.Equal(t, ReconIssueStalePrice, issue.Type)
				require.Equal(t, ReconSeverityDrift, issue.Severity)
				require.Equal(t, "NSC", issue.Symbol)
				requireQtyFields(t, issue, 1, 1, 0)
				require.NotNil(t, issue.DollarValue)
				require.InDelta(t, 200, *issue.DollarValue, 1e-9)
			},
		},
		{
			name: "price on the cutoff day is not stale",
			in: aggregateReconcileInput{
				LedgerQty: map[string]decimal.Decimal{"NSC": dec("1")},
				AlpacaQty: map[string]decimal.Decimal{"NSC": dec("1")},
				Prices: map[string]domain.AssetPrice{
					"NSC": priceAt("NSC", "200", staleBefore),
				},
				StaleBefore: staleBefore,
			},
			wantStatus: ReconStatusOK,
			wantCount:  0,
		},
		{
			name: "stale dust holding stays DUST so it cannot hide as OK",
			in: aggregateReconcileInput{
				LedgerQty: map[string]decimal.Decimal{"RMD": dec("0.10")},
				AlpacaQty: map[string]decimal.Decimal{},
				Prices: map[string]domain.AssetPrice{
					"RMD": priceAt("RMD", "100", testDate(-10)),
				},
				StaleBefore: staleBefore,
			},
			wantStatus: ReconStatusDustOnly,
			wantCount:  2,
			assert: func(t *testing.T, issues []ReconIssue) {
				phantom := issueByTypeSymbol(t, issues, ReconIssuePhantomPosition, "RMD")
				require.Equal(t, ReconSeverityDust, phantom.Severity)
				stale := issueByTypeSymbol(t, issues, ReconIssueStalePrice, "RMD")
				require.Equal(t, ReconSeverityDust, stale.Severity)
			},
		},
		{
			name: "full union reports both phantom ledger symbols and untracked alpaca symbols",
			in: aggregateReconcileInput{
				LedgerQty: map[string]decimal.Decimal{"CEG": dec("0.05")},
				AlpacaQty: map[string]decimal.Decimal{"NVDA": dec("0.01")},
				Prices: map[string]domain.AssetPrice{
					"CEG":  priceAt("CEG", "200", fresh),
					"NVDA": priceAt("NVDA", "100", fresh),
				},
				StaleBefore: staleBefore,
			},
			wantStatus: ReconStatusDustOnly,
			wantCount:  2,
			assert: func(t *testing.T, issues []ReconIssue) {
				issueByTypeSymbol(t, issues, ReconIssuePhantomPosition, "CEG")
				issueByTypeSymbol(t, issues, ReconIssueUntrackedPosition, "NVDA")
			},
		},
		{
			name: "DRIFT qty mismatch wins status over DUST cash",
			in: aggregateReconcileInput{
				LedgerCash: dec("10.00"),
				AlpacaCash: dec("9.00"),
				LedgerQty:  map[string]decimal.Decimal{"AAPL": dec("10")},
				AlpacaQty:  map[string]decimal.Decimal{"AAPL": dec("1")},
				Prices: map[string]domain.AssetPrice{
					"AAPL": priceAt("AAPL", "150", fresh),
				},
				StaleBefore: staleBefore,
			},
			wantStatus: ReconStatusDrift,
			wantCount:  2,
			assert: func(t *testing.T, issues []ReconIssue) {
				require.Equal(t, ReconIssueCashMismatch, issues[0].Type)
				require.Equal(t, ReconSeverityDust, issues[0].Severity)
				require.Equal(t, ReconIssueQuantityMismatch, issues[1].Type)
				require.Equal(t, ReconSeverityDrift, issues[1].Severity)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.in.LedgerQty == nil {
				tt.in.LedgerQty = map[string]decimal.Decimal{}
			}
			if tt.in.AlpacaQty == nil {
				tt.in.AlpacaQty = map[string]decimal.Decimal{}
			}
			issues := compareAggregatePortfolios(tt.in)
			require.Len(t, issues, tt.wantCount)
			require.Equal(t, tt.wantStatus, reconcileStatus(issues))
			if tt.assert != nil {
				tt.assert(t, issues)
			}
		})
	}
}

func TestStalePriceCutoff(t *testing.T) {
	now := testDate(0)
	days := []time.Time{
		testDate(-10),
		testDate(-9),
		testDate(-8),
		testDate(-7),
		testDate(-6),
		testDate(-3),
		now,
	}

	t.Run("uses the trading day n sessions before the latest", func(t *testing.T) {
		cutoff := stalePriceCutoff(now, days, 5)
		require.True(t, cutoff.Equal(testDate(-9)))
	})

	t.Run("falls back to calendar days when history is short", func(t *testing.T) {
		cutoff := stalePriceCutoff(now, days[:3], 5)
		require.True(t, cutoff.Equal(testDate(-5)))
	})
}

func TestReconcileStatus(t *testing.T) {
	tests := []struct {
		name   string
		issues []ReconIssue
		want   string
	}{
		{name: "empty is OK", want: ReconStatusOK},
		{
			name:   "all dust is DUST_ONLY",
			issues: []ReconIssue{{Severity: ReconSeverityDust}, {Severity: ReconSeverityDust}},
			want:   ReconStatusDustOnly,
		},
		{
			name:   "any drift is DRIFT",
			issues: []ReconIssue{{Severity: ReconSeverityDust}, {Severity: ReconSeverityDrift}},
			want:   ReconStatusDrift,
		},
		{
			name:   "empty severity is treated as DRIFT",
			issues: []ReconIssue{{Message: "per-investment issue"}},
			want:   ReconStatusDrift,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, reconcileStatus(tt.issues))
		})
	}
}

func expectFreshPrices(priceRepo *mock_repository.MockAdjustedPriceRepository, prices map[string]domain.AssetPrice) {
	symbols := make([]string, 0, len(prices))
	for symbol := range prices {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)
	dates := map[string]time.Time{}
	list := make([]domain.AssetPrice, 0, len(symbols))
	for _, symbol := range symbols {
		dates[symbol] = prices[symbol].Date
		list = append(list, prices[symbol])
	}
	priceRepo.EXPECT().LatestPriceDates(symbols).Return(dates, nil)
	priceRepo.EXPECT().LatestPrices(symbols).Return(list, nil)
	priceRepo.EXPECT().ListTradingDays(gomock.Any(), gomock.Any()).Return([]time.Time{time.Now().UTC()}, nil)
}

func TestReconcileAggregatePortfolioIncludesClosedInvestments(t *testing.T) {
	ctrl := gomock.NewController(t)
	investmentRepository := mock_repository.NewMockInvestmentRepository(ctrl)
	holdingsRepository := mock_repository.NewMockInvestmentHoldingsRepository(ctrl)
	alpacaRepository := mock_repository.NewMockAlpacaRepository(ctrl)
	priceRepository := mock_repository.NewMockAdjustedPriceRepository(ctrl)

	activeID := uuid.New()
	closedID := uuid.MustParse("947b6eff-0000-0000-0000-000000000001")
	cash := decimal.NewFromInt(10)
	closedQty := dec("0.12")
	now := time.Now().UTC()

	handler := investmentServiceHandler{
		InvestmentRepository: investmentRepository,
		HoldingsRepository:   holdingsRepository,
		AlpacaRepository:     alpacaRepository,
		PriceRepository:      priceRepository,
	}

	investmentRepository.EXPECT().
		List(repository.StrategyInvestmentListFilter{IncludePaused: true, IncludeEnded: true}).
		Return([]model.Investment{
			{InvestmentID: activeID},
			{InvestmentID: closedID, EndDate: &now},
		}, nil)
	holdingsRepository.EXPECT().
		GetLatestHoldings(nil, activeID).
		Return(&domain.Portfolio{
			Cash:      &cash,
			Positions: map[string]*domain.Position{},
		}, nil)
	holdingsRepository.EXPECT().
		GetLatestHoldings(nil, closedID).
		Return(&domain.Portfolio{
			Cash: &cash,
			Positions: map[string]*domain.Position{
				"VST": {Symbol: "VST", ExactQuantity: closedQty},
			},
		}, nil)
	alpacaRepository.EXPECT().GetAccount().Return(&alpaca.Account{Cash: cash.Add(cash)}, nil)
	alpacaRepository.EXPECT().GetPositions().Return([]alpaca.Position{}, nil)
	expectFreshPrices(priceRepository, map[string]domain.AssetPrice{
		"VST": priceAt("VST", "100", now),
	})

	issues, err := handler.reconcileAggregatePortfolio()
	require.NoError(t, err)
	require.Len(t, issues, 1)
	require.Equal(t, ReconIssuePhantomPosition, issues[0].Type)
	require.Equal(t, ReconSeverityDust, issues[0].Severity)
	require.Equal(t, "VST", issues[0].Symbol)
	requireQtyFields(t, issues[0], 0.12, 0, -0.12)
}

func TestReconcileAggregatePortfolioINTCShortage(t *testing.T) {
	ctrl := gomock.NewController(t)
	investmentRepository := mock_repository.NewMockInvestmentRepository(ctrl)
	holdingsRepository := mock_repository.NewMockInvestmentHoldingsRepository(ctrl)
	alpacaRepository := mock_repository.NewMockAlpacaRepository(ctrl)
	priceRepository := mock_repository.NewMockAdjustedPriceRepository(ctrl)

	investmentID := uuid.New()
	ledgerQty := dec("1.197")
	brokerQty := dec("0.764")
	cash := decimal.NewFromInt(100)
	now := time.Now().UTC()

	handler := investmentServiceHandler{
		InvestmentRepository: investmentRepository,
		HoldingsRepository:   holdingsRepository,
		AlpacaRepository:     alpacaRepository,
		PriceRepository:      priceRepository,
	}

	investmentRepository.EXPECT().
		List(repository.StrategyInvestmentListFilter{IncludePaused: true, IncludeEnded: true}).
		Return([]model.Investment{{InvestmentID: investmentID}}, nil)
	holdingsRepository.EXPECT().
		GetLatestHoldings(nil, investmentID).
		Return(&domain.Portfolio{
			Cash: &cash,
			Positions: map[string]*domain.Position{
				"INTC": {
					Symbol:        "INTC",
					ExactQuantity: ledgerQty,
				},
			},
		}, nil)
	alpacaRepository.EXPECT().GetAccount().Return(&alpaca.Account{Cash: cash}, nil)
	alpacaRepository.EXPECT().GetPositions().Return([]alpaca.Position{
		{Symbol: "INTC", Qty: brokerQty},
	}, nil)
	expectFreshPrices(priceRepository, map[string]domain.AssetPrice{
		"INTC": priceAt("INTC", "20", now),
	})

	issues, err := handler.reconcileAggregatePortfolio()
	require.NoError(t, err)
	require.Len(t, issues, 1)
	require.Equal(t, ReconIssueQuantityMismatch, issues[0].Type)
	require.Equal(t, ReconSeverityDust, issues[0].Severity)
	require.Equal(t, "INTC", issues[0].Symbol)
	require.Nil(t, issues[0].InvestmentID)
	requireQtyFields(t, issues[0], 1.197, 0.764, -0.433)
}

func TestReconErrToIssueAggregateScope(t *testing.T) {
	issue := reconErrToIssue(ReconErr{
		Message: "alpaca account holding insufficient INTC: aggregate portfolio 1.197000 vs alpaca 0.764000 (-0.433000)",
	})
	require.Nil(t, issue.InvestmentID)
	require.Equal(t, ReconSeverityDrift, issue.Severity)
	require.Contains(t, issue.Message, "INTC")
}

func TestReconcileNoInvestmentsDoesNotPanic(t *testing.T) {
	ctrl := gomock.NewController(t)
	investmentRepository := mock_repository.NewMockInvestmentRepository(ctrl)
	alpacaRepository := mock_repository.NewMockAlpacaRepository(ctrl)

	handler := investmentServiceHandler{
		InvestmentRepository: investmentRepository,
		AlpacaRepository:     alpacaRepository,
	}

	investmentRepository.EXPECT().
		List(gomock.Any()).
		Return([]model.Investment{}, nil).
		Times(2)
	alpacaRepository.EXPECT().GetAccount().Return(&alpaca.Account{Cash: decimal.Zero}, nil)
	alpacaRepository.EXPECT().GetPositions().Return([]alpaca.Position{}, nil)

	require.NotPanics(t, func() {
		err := handler.Reconcile(context.Background())
		require.NoError(t, err)
	})
}

func TestRunReconcileNoInvestments(t *testing.T) {
	ctrl := gomock.NewController(t)
	investmentRepository := mock_repository.NewMockInvestmentRepository(ctrl)
	alpacaRepository := mock_repository.NewMockAlpacaRepository(ctrl)

	handler := investmentServiceHandler{
		InvestmentRepository: investmentRepository,
		AlpacaRepository:     alpacaRepository,
	}

	investmentRepository.EXPECT().
		List(gomock.Any()).
		Return([]model.Investment{}, nil).
		Times(2)
	alpacaRepository.EXPECT().GetAccount().Return(&alpaca.Account{Cash: decimal.Zero}, nil)
	alpacaRepository.EXPECT().GetPositions().Return([]alpaca.Position{}, nil)

	result, err := handler.RunReconcile(context.Background())
	require.NoError(t, err)
	require.Equal(t, ReconStatusOK, result.Status)
	require.Empty(t, result.Issues)
}

func TestLogPreRebalanceReconcile(t *testing.T) {
	t.Run("continues with empty drift when reconcile errors", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		investmentRepository := mock_repository.NewMockInvestmentRepository(ctrl)
		handler := investmentServiceHandler{InvestmentRepository: investmentRepository}
		investmentRepository.EXPECT().
			List(gomock.Any()).
			Return(nil, context.DeadlineExceeded)

		issues := handler.logPreRebalanceReconcile(context.Background())
		require.Empty(t, issues)
	})

	t.Run("omits DUST from returned drift issues", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		investmentRepository := mock_repository.NewMockInvestmentRepository(ctrl)
		holdingsRepository := mock_repository.NewMockInvestmentHoldingsRepository(ctrl)
		alpacaRepository := mock_repository.NewMockAlpacaRepository(ctrl)
		priceRepository := mock_repository.NewMockAdjustedPriceRepository(ctrl)

		closedID := uuid.MustParse("2bf80abe-0000-0000-0000-000000000001")
		cash := decimal.Zero
		now := time.Now().UTC()
		handler := investmentServiceHandler{
			InvestmentRepository: investmentRepository,
			HoldingsRepository:   holdingsRepository,
			AlpacaRepository:     alpacaRepository,
			PriceRepository:      priceRepository,
		}

		investmentRepository.EXPECT().
			List(repository.StrategyInvestmentListFilter{}).
			Return([]model.Investment{}, nil)
		investmentRepository.EXPECT().
			List(repository.StrategyInvestmentListFilter{IncludePaused: true, IncludeEnded: true}).
			Return([]model.Investment{{InvestmentID: closedID, EndDate: &now}}, nil)
		holdingsRepository.EXPECT().
			GetLatestHoldings(nil, closedID).
			Return(&domain.Portfolio{
				Cash: &cash,
				Positions: map[string]*domain.Position{
					"GEV": {Symbol: "GEV", ExactQuantity: dec("0.10")},
				},
			}, nil)
		alpacaRepository.EXPECT().GetAccount().Return(&alpaca.Account{Cash: cash}, nil)
		alpacaRepository.EXPECT().GetPositions().Return([]alpaca.Position{}, nil)
		expectFreshPrices(priceRepository, map[string]domain.AssetPrice{
			"GEV": priceAt("GEV", "100", now),
		})

		issues := handler.logPreRebalanceReconcile(context.Background())
		require.Empty(t, issues)
	})

	t.Run("returns DRIFT issues", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		investmentRepository := mock_repository.NewMockInvestmentRepository(ctrl)
		holdingsRepository := mock_repository.NewMockInvestmentHoldingsRepository(ctrl)
		alpacaRepository := mock_repository.NewMockAlpacaRepository(ctrl)
		priceRepository := mock_repository.NewMockAdjustedPriceRepository(ctrl)

		handler := investmentServiceHandler{
			InvestmentRepository: investmentRepository,
			HoldingsRepository:   holdingsRepository,
			AlpacaRepository:     alpacaRepository,
			PriceRepository:      priceRepository,
		}
		now := time.Now().UTC()
		cash := decimal.Zero

		investmentRepository.EXPECT().
			List(repository.StrategyInvestmentListFilter{}).
			Return([]model.Investment{}, nil)
		investmentRepository.EXPECT().
			List(repository.StrategyInvestmentListFilter{IncludePaused: true, IncludeEnded: true}).
			Return([]model.Investment{}, nil)
		alpacaRepository.EXPECT().GetAccount().Return(&alpaca.Account{Cash: cash}, nil)
		alpacaRepository.EXPECT().GetPositions().Return([]alpaca.Position{
			{Symbol: "NVDA", Qty: dec("3")},
		}, nil)
		expectFreshPrices(priceRepository, map[string]domain.AssetPrice{
			"NVDA": priceAt("NVDA", "100", now),
		})

		issues := handler.logPreRebalanceReconcile(context.Background())
		require.Len(t, issues, 1)
		require.Equal(t, ReconIssueUntrackedPosition, issues[0].Type)
		require.Equal(t, ReconSeverityDrift, issues[0].Severity)
		require.Equal(t, "NVDA", issues[0].Symbol)
	})
}

func TestRunReconcileDoesNotPanicWithoutProfileInContext(t *testing.T) {
	ctrl := gomock.NewController(t)

	investmentRepository := mock_repository.NewMockInvestmentRepository(ctrl)
	strategyRepository := mock_repository.NewMockStrategyRepository(ctrl)
	holdingsRepository := mock_repository.NewMockInvestmentHoldingsRepository(ctrl)
	holdingsVersionRepository := mock_repository.NewMockInvestmentHoldingsVersionRepository(ctrl)
	investmentTradeRepository := mock_repository.NewMockInvestmentTradeRepository(ctrl)
	universeRepository := mock_repository.NewMockAssetUniverseRepository(ctrl)
	priceRepository := mock_repository.NewMockAdjustedPriceRepository(ctrl)
	alpacaRepository := mock_repository.NewMockAlpacaRepository(ctrl)

	investmentID := uuid.New()
	strategyID := uuid.New()
	versionID := uuid.New()
	cash := decimal.NewFromInt(100)
	portfolio := &domain.Portfolio{
		Cash:      &cash,
		Positions: map[string]*domain.Position{},
	}
	investment := model.Investment{
		InvestmentID:  investmentID,
		StrategyID:    strategyID,
		StartDate:     time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		AmountDollars: 1000,
	}
	strategy := model.Strategy{
		StrategyID:       strategyID,
		FactorExpression: "close",
		NumAssets:        1,
		AssetUniverse:    "sp500",
	}

	handler := investmentServiceHandler{
		InvestmentRepository:      investmentRepository,
		StrategyRepository:        strategyRepository,
		HoldingsRepository:        holdingsRepository,
		HoldingsVersionRepository: holdingsVersionRepository,
		InvestmentTradeRepository: investmentTradeRepository,
		UniverseRepository:        universeRepository,
		AlpacaRepository:          alpacaRepository,
		BacktestHandler: BacktestHandler{
			AssetUniverseRepository: universeRepository,
			PriceRepository:         priceRepository,
		},
	}

	investmentRepository.EXPECT().
		List(gomock.Any()).
		Return([]model.Investment{investment}, nil).
		Times(2)
	investmentRepository.EXPECT().
		Get(investmentID).
		Return(&investment, nil)
	strategyRepository.EXPECT().
		Get(strategyID).
		Return(&strategy, nil)
	universeRepository.EXPECT().
		GetAssets("sp500").
		Return([]model.Ticker{{Symbol: "AAPL"}}, nil)
	priceRepository.EXPECT().
		ListTradingDays(gomock.Any(), gomock.Any()).
		Return([]time.Time{}, nil)
	holdingsRepository.EXPECT().
		GetLatestHoldings(nil, investmentID).
		Return(portfolio, nil).
		Times(3)
	holdingsVersionRepository.EXPECT().
		GetLatestNotedVersion(investmentID).
		Return(nil, nil)
	holdingsVersionRepository.EXPECT().
		GetEarliestVersionID(investmentID).
		Return(&versionID, nil)
	holdingsRepository.EXPECT().
		Get(versionID).
		Return(portfolio, nil)
	investmentTradeRepository.EXPECT().
		List(nil, gomock.Any()).
		Return([]*model.InvestmentTradeStatus{}, nil)
	alpacaRepository.EXPECT().GetAccount().Return(&alpaca.Account{Cash: cash}, nil)
	alpacaRepository.EXPECT().GetPositions().Return([]alpaca.Position{}, nil)

	require.NotPanics(t, func() {
		result, err := handler.RunReconcile(context.Background())
		require.NoError(t, err)
		require.Equal(t, ReconStatusOK, result.Status)
	})
}
