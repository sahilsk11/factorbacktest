package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"factorbacktest/internal/domain"
	"factorbacktest/internal/logger"
	"factorbacktest/internal/repository"

	"github.com/shopspring/decimal"
)

const (
	ReconIssuePhantomPosition   = "PHANTOM_POSITION"
	ReconIssueUntrackedPosition = "UNTRACKED_POSITION"
	ReconIssueQuantityMismatch  = "QUANTITY_MISMATCH"
	ReconIssueCashMismatch      = "CASH_MISMATCH"
	ReconIssueStalePrice        = "STALE_PRICE"

	ReconSeverityDust  = "DUST"
	ReconSeverityDrift = "DRIFT"

	ReconStatusOK       = "OK"
	ReconStatusDustOnly = "DUST_ONLY"
	ReconStatusDrift    = "DRIFT"

	ReconCashSymbol = ":CASH"

	// reconStalePriceTradingDays is how old an adjusted price may be before
	// a held symbol is flagged STALE_PRICE.
	reconStalePriceTradingDays = 5
	// reconStalePriceLookbackCalendarDays bounds the trading-day query used
	// to compute that cutoff (5 trading days plus weekends/holidays).
	reconStalePriceLookbackCalendarDays = 30
)

var (
	// reconQuantityEpsilon is the share-qty noise floor. Differences at or
	// below this are treated as a match so float/decimal residue does not
	// create issues; anything above is reported (including fractional dust).
	reconQuantityEpsilon = decimal.RequireFromString("0.000001")
	// reconCashTolerance is the dollar slack allowed between ledger and
	// Alpaca cash before emitting CASH_MISMATCH.
	reconCashTolerance = decimal.RequireFromString("0.50")
	// reconDustDollarThreshold classifies an issue as DUST when the absolute
	// dollar impact is strictly below this amount, and DRIFT otherwise.
	reconDustDollarThreshold = decimal.RequireFromString("50")
)

type ReconcileResult struct {
	Status    string       `json:"status"`
	CheckedAt time.Time    `json:"checkedAt"`
	Issues    []ReconIssue `json:"issues"`
}

type RebalanceResult struct {
	DriftIssues []ReconIssue `json:"driftIssues"`
}

type ReconIssue struct {
	Type         string   `json:"type,omitempty"`
	Severity     string   `json:"severity,omitempty"`
	Symbol       string   `json:"symbol,omitempty"`
	LedgerQty    *float64 `json:"ledgerQty,omitempty"`
	AlpacaQty    *float64 `json:"alpacaQty,omitempty"`
	Diff         *float64 `json:"diff,omitempty"`
	DollarValue  *float64 `json:"dollarValue,omitempty"`
	Message      string   `json:"message"`
	InvestmentID *string  `json:"investmentId,omitempty"`
}

type aggregateReconcileInput struct {
	LedgerCash  decimal.Decimal
	AlpacaCash  decimal.Decimal
	LedgerQty   map[string]decimal.Decimal
	AlpacaQty   map[string]decimal.Decimal
	Prices      map[string]domain.AssetPrice
	StaleBefore time.Time
}

func reconErrToIssue(err ReconErr) ReconIssue {
	issue := ReconIssue{
		Message:  err.Message,
		Severity: ReconSeverityDrift,
	}
	if err.InvestmentID != nil {
		investmentID := err.InvestmentID.String()
		issue.InvestmentID = &investmentID
	}
	return issue
}

func reconcileStatus(issues []ReconIssue) string {
	if len(issues) == 0 {
		return ReconStatusOK
	}
	for _, issue := range issues {
		if issue.Severity != ReconSeverityDust {
			return ReconStatusDrift
		}
	}
	return ReconStatusDustOnly
}

func (h investmentServiceHandler) RunReconcile(ctx context.Context) (*ReconcileResult, error) {
	profile, endProfile := domain.NewProfile()
	defer endProfile()
	ctx = context.WithValue(ctx, domain.ContextProfileKey, profile)

	investments, err := h.InvestmentRepository.List(repository.StrategyInvestmentListFilter{})
	if err != nil {
		return nil, err
	}

	issues := make([]ReconIssue, 0)
	for _, investment := range investments {
		reconErrors, err := h.reconcileInvestment(ctx, investment.InvestmentID)
		if err != nil {
			return nil, err
		}
		for _, reconErr := range reconErrors {
			issues = append(issues, reconErrToIssue(reconErr))
		}

		reconErr, err := h.reconcileTrades(investment.InvestmentID)
		if err != nil {
			return nil, err
		}
		if reconErr != nil {
			issues = append(issues, reconErrToIssue(*reconErr))
		}
	}

	aggregateIssues, err := h.reconcileAggregatePortfolio()
	if err != nil {
		return nil, err
	}
	issues = append(issues, aggregateIssues...)

	return &ReconcileResult{
		Status:    reconcileStatus(issues),
		CheckedAt: time.Now().UTC(),
		Issues:    issues,
	}, nil
}

func (h investmentServiceHandler) Reconcile(ctx context.Context) error {
	log := logger.FromContext(ctx)
	result, err := h.RunReconcile(ctx)
	if err != nil {
		return err
	}

	for _, issue := range result.Issues {
		if issue.InvestmentID != nil {
			log.Warnf("recon err on investment %s: %s", *issue.InvestmentID, issue.Message)
			continue
		}
		log.Warnf("recon err on aggregate portfolio: %s", issue.Message)
	}

	return nil
}

// logPreRebalanceReconcile runs the read-only Alpaca-vs-ledger reconcile
// before a rebalance. Failures and drift are logged only — this never
// mutates holdings, places orders, or aborts rebalancing.
func (h investmentServiceHandler) logPreRebalanceReconcile(ctx context.Context) []ReconIssue {
	log := logger.FromContext(ctx)
	result, err := h.RunReconcile(ctx)
	if err != nil {
		log.Warnf("pre-rebalance reconcile failed (continuing rebalance): %s", err.Error())
		return []ReconIssue{}
	}

	drift := make([]ReconIssue, 0)
	for _, issue := range result.Issues {
		if issue.Severity == ReconSeverityDrift {
			log.Warnf("pre-rebalance DRIFT: %s", issue.Message)
			drift = append(drift, issue)
			continue
		}
		if issue.Severity == ReconSeverityDust {
			log.Infof("pre-rebalance DUST: %s", issue.Message)
		}
	}
	return drift
}

func (h investmentServiceHandler) reconcileAggregatePortfolio() ([]ReconIssue, error) {
	investments, err := h.InvestmentRepository.List(repository.StrategyInvestmentListFilter{
		IncludePaused: true,
		IncludeEnded:  true,
	})
	if err != nil {
		return nil, err
	}

	ledgerQty := map[string]decimal.Decimal{}
	ledgerCash := decimal.Zero
	for _, i := range investments {
		holdings, err := h.HoldingsRepository.GetLatestHoldings(nil, i.InvestmentID)
		if err != nil {
			return nil, err
		}
		if holdings != nil && holdings.Cash != nil {
			ledgerCash = ledgerCash.Add(*holdings.Cash)
		}
		if holdings == nil {
			continue
		}
		for _, p := range holdings.Positions {
			ledgerQty[p.Symbol] = ledgerQty[p.Symbol].Add(p.ExactQuantity)
		}
	}

	account, err := h.AlpacaRepository.GetAccount()
	if err != nil {
		return nil, err
	}

	actuallyHeld, err := h.AlpacaRepository.GetPositions()
	if err != nil {
		return nil, err
	}
	alpacaQty := map[string]decimal.Decimal{}
	for _, p := range actuallyHeld {
		alpacaQty[p.Symbol] = alpacaQty[p.Symbol].Add(p.Qty)
	}

	symbols := unionSymbols(ledgerQty, alpacaQty)
	now := time.Now().UTC()
	prices := map[string]domain.AssetPrice{}
	staleBefore := time.Time{}
	if len(symbols) > 0 {
		var priceErr error
		prices, priceErr = h.latestAdjustedPrices(symbols)
		if priceErr != nil {
			return nil, priceErr
		}
		staleBefore, priceErr = h.stalePriceCutoffAt(now)
		if priceErr != nil {
			return nil, priceErr
		}
	}

	return compareAggregatePortfolios(aggregateReconcileInput{
		LedgerCash:  ledgerCash,
		AlpacaCash:  account.Cash,
		LedgerQty:   ledgerQty,
		AlpacaQty:   alpacaQty,
		Prices:      prices,
		StaleBefore: staleBefore,
	}), nil
}

func (h investmentServiceHandler) latestAdjustedPrices(symbols []string) (map[string]domain.AssetPrice, error) {
	out := map[string]domain.AssetPrice{}
	if len(symbols) == 0 {
		return out, nil
	}

	dates, err := h.PriceRepository.LatestPriceDates(symbols)
	if err != nil {
		return nil, err
	}

	found := make([]string, 0, len(dates))
	for _, symbol := range symbols {
		if _, ok := dates[symbol]; ok {
			found = append(found, symbol)
		}
	}
	if len(found) == 0 {
		return out, nil
	}

	prices, err := h.PriceRepository.LatestPrices(found)
	if err != nil {
		return nil, err
	}
	for _, p := range prices {
		out[p.Symbol] = p
	}
	return out, nil
}

func (h investmentServiceHandler) stalePriceCutoffAt(now time.Time) (time.Time, error) {
	start := dateOnlyUTC(now).AddDate(0, 0, -reconStalePriceLookbackCalendarDays)
	days, err := h.PriceRepository.ListTradingDays(start, now)
	if err != nil {
		return time.Time{}, err
	}
	return stalePriceCutoff(now, days, reconStalePriceTradingDays), nil
}

func compareAggregatePortfolios(in aggregateReconcileInput) []ReconIssue {
	issues := make([]ReconIssue, 0)

	cashDiff := in.AlpacaCash.Sub(in.LedgerCash)
	if cashDiff.Abs().GreaterThan(reconCashTolerance) {
		issues = append(issues, newCashIssue(in.LedgerCash, in.AlpacaCash, cashDiff))
	}

	for _, symbol := range unionSymbols(in.LedgerQty, in.AlpacaQty) {
		ledgerQty := in.LedgerQty[symbol]
		alpacaQty := in.AlpacaQty[symbol]
		ledgerZero := isEffectivelyZero(ledgerQty)
		alpacaZero := isEffectivelyZero(alpacaQty)
		if ledgerZero && alpacaZero {
			continue
		}

		price, hasPrice := in.Prices[symbol]
		var pricePtr *domain.AssetPrice
		if hasPrice {
			priceCopy := price
			pricePtr = &priceCopy
		}

		diff := alpacaQty.Sub(ledgerQty)
		if ledgerZero {
			issues = append(issues, newQtyIssue(ReconIssueUntrackedPosition, symbol, ledgerQty, alpacaQty, diff, pricePtr))
		} else if alpacaZero {
			issues = append(issues, newQtyIssue(ReconIssuePhantomPosition, symbol, ledgerQty, alpacaQty, diff, pricePtr))
		} else if diff.Abs().GreaterThan(reconQuantityEpsilon) {
			issues = append(issues, newQtyIssue(ReconIssueQuantityMismatch, symbol, ledgerQty, alpacaQty, diff, pricePtr))
		}

		if isPriceStale(pricePtr, hasPrice, in.StaleBefore) {
			issues = append(issues, newStalePriceIssue(symbol, ledgerQty, alpacaQty, pricePtr, hasPrice, in.StaleBefore))
		}
	}

	return issues
}

func newCashIssue(ledgerCash, alpacaCash, diff decimal.Decimal) ReconIssue {
	return ReconIssue{
		Type:        ReconIssueCashMismatch,
		Severity:    severityFromAbsDollars(diff.Abs()),
		Symbol:      ReconCashSymbol,
		LedgerQty:   floatPtr(ledgerCash),
		AlpacaQty:   floatPtr(alpacaCash),
		Diff:        floatPtr(diff),
		DollarValue: floatPtr(diff),
		Message: fmt.Sprintf(
			"%s: ledger %s vs alpaca %s (diff %s)",
			ReconIssueCashMismatch,
			ledgerCash.String(),
			alpacaCash.String(),
			diff.String(),
		),
	}
}

func newQtyIssue(
	issueType, symbol string,
	ledgerQty, alpacaQty, diff decimal.Decimal,
	price *domain.AssetPrice,
) ReconIssue {
	issue := ReconIssue{
		Type:      issueType,
		Severity:  ReconSeverityDrift,
		Symbol:    symbol,
		LedgerQty: floatPtr(ledgerQty),
		AlpacaQty: floatPtr(alpacaQty),
		Diff:      floatPtr(diff),
		Message: fmt.Sprintf(
			"%s %s: ledger %s vs alpaca %s (diff %s)",
			issueType,
			symbol,
			ledgerQty.String(),
			alpacaQty.String(),
			diff.String(),
		),
	}
	if price != nil {
		dollar := diff.Mul(price.Price)
		issue.DollarValue = floatPtr(dollar)
		issue.Severity = severityFromAbsDollars(dollar.Abs())
		issue.Message += fmt.Sprintf(" (~$%s)", dollar.Abs().StringFixed(2))
	}
	return issue
}

func newStalePriceIssue(
	symbol string,
	ledgerQty, alpacaQty decimal.Decimal,
	price *domain.AssetPrice,
	hasPrice bool,
	staleBefore time.Time,
) ReconIssue {
	diff := alpacaQty.Sub(ledgerQty)
	issue := ReconIssue{
		Type:      ReconIssueStalePrice,
		Severity:  ReconSeverityDrift,
		Symbol:    symbol,
		LedgerQty: floatPtr(ledgerQty),
		AlpacaQty: floatPtr(alpacaQty),
		Diff:      floatPtr(diff),
	}
	if !hasPrice || price == nil {
		issue.Message = fmt.Sprintf("%s %s: no adjusted price found", ReconIssueStalePrice, symbol)
		return issue
	}

	holdingQty := ledgerQty.Abs()
	if alpacaQty.Abs().GreaterThan(holdingQty) {
		holdingQty = alpacaQty.Abs()
	}
	holdingValue := holdingQty.Mul(price.Price)
	issue.DollarValue = floatPtr(holdingValue)
	issue.Severity = severityFromAbsDollars(holdingValue)
	issue.Message = fmt.Sprintf(
		"%s %s: latest adjusted price %s is older than %d trading days (cutoff %s)",
		ReconIssueStalePrice,
		symbol,
		dateOnlyUTC(price.Date).Format(time.DateOnly),
		reconStalePriceTradingDays,
		dateOnlyUTC(staleBefore).Format(time.DateOnly),
	)
	return issue
}

func isPriceStale(price *domain.AssetPrice, hasPrice bool, staleBefore time.Time) bool {
	if staleBefore.IsZero() {
		return !hasPrice || price == nil
	}
	if !hasPrice || price == nil {
		return true
	}
	return dateOnlyUTC(price.Date).Before(dateOnlyUTC(staleBefore))
}

func severityFromAbsDollars(absDollars decimal.Decimal) string {
	if absDollars.LessThan(reconDustDollarThreshold) {
		return ReconSeverityDust
	}
	return ReconSeverityDrift
}

func isEffectivelyZero(qty decimal.Decimal) bool {
	return qty.Abs().LessThanOrEqual(reconQuantityEpsilon)
}

func unionSymbols(a, b map[string]decimal.Decimal) []string {
	seen := map[string]struct{}{}
	for symbol := range a {
		seen[symbol] = struct{}{}
	}
	for symbol := range b {
		seen[symbol] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for symbol := range seen {
		out = append(out, symbol)
	}
	sort.Strings(out)
	return out
}

func stalePriceCutoff(now time.Time, tradingDays []time.Time, n int) time.Time {
	now = dateOnlyUTC(now)
	if n <= 0 {
		return now
	}
	if len(tradingDays) > n {
		return dateOnlyUTC(tradingDays[len(tradingDays)-1-n])
	}
	return now.AddDate(0, 0, -n)
}

func dateOnlyUTC(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func floatPtr(d decimal.Decimal) *float64 {
	f := d.InexactFloat64()
	return &f
}
