package l2_service

import (
	"context"
	"database/sql"
	l1_service "factorbacktest/internal/service/l1"

	"fmt"
	"math"
	"time"

	"github.com/maja42/goval"
)

const dateLayout = "2006-01-02"

func constructFunctionMap(
	ctx context.Context,
	db *sql.DB,
	pr *l1_service.PriceCache,
	symbol string,
	h FactorMetricCalculations,
	debug FormulaDebugger,
	currentDate time.Time,
) map[string]goval.ExpressionFunction {
	return map[string]goval.ExpressionFunction{
		// we could break this up

		// helper functions
		"addDate": func(args ...interface{}) (interface{}, error) {
			// addDate(date, years, months, days)
			if len(args) < 4 {
				return 0, fmt.Errorf("addDate needs needed 4 args, got %d", len(args))
			}
			date, err := time.Parse(dateLayout, args[0].(string))
			if err != nil {
				return 0, err
			}

			var years, months, days = args[1].(int), args[2].(int), args[3].(int)

			date = date.AddDate(years, months, days)

			return date.Format(dateLayout), nil
		},

		"nDaysAgo": func(args ...interface{}) (interface{}, error) {
			if len(args) < 1 {
				return 0, fmt.Errorf("nDaysAgo needs needed 1 arg, got %d", len(args))
			}
			n := args[0].(int)
			d := currentDate.AddDate(0, 0, -n)

			return d.Format(dateLayout), nil
		},
		"nMonthsAgo": func(args ...interface{}) (interface{}, error) {
			if len(args) < 1 {
				return 0, fmt.Errorf("nMonthsAgo needs needed 1 arg, got %d", len(args))
			}
			n := args[0].(int)
			d := currentDate.AddDate(0, -n, 0)

			return d.Format(dateLayout), nil
		},
		"nYearsAgo": func(args ...interface{}) (interface{}, error) {
			if len(args) < 1 {
				return 0, fmt.Errorf("nYearsAgo needs needed 1 arg, got %d", len(args))
			}
			n := args[0].(int)
			d := currentDate.AddDate(-n, 0, 0)

			return d.Format(dateLayout), nil
		},

		// metric functions

		// price(date strDate)
		"price": func(args ...interface{}) (interface{}, error) {
			if len(args) < 1 {
				return 0, fmt.Errorf("price needs needed 1 arg, got %d", len(args))
			}
			date, err := time.Parse(dateLayout, args[0].(string))
			if err != nil {
				return 0, err
			}
			p, err := h.Price(pr, symbol, date)
			if err != nil {
				return 0, err
			}

			debug.Add("price", p)

			return p, nil
		},

		// pricePercentChange(start, end strDate)
		"pricePercentChange": func(args ...interface{}) (interface{}, error) {
			if len(args) < 2 {
				return 0, fmt.Errorf("pricePercentChange needs needed 2 args, got %d", len(args))
			}
			start, err := time.Parse(dateLayout, args[0].(string))
			if err != nil {
				return 0, err
			}
			end, err := time.Parse(dateLayout, args[1].(string))
			if err != nil {
				return 0, err
			}

			p, err := h.PricePercentChange(pr, symbol, start, end)
			if err != nil {
				return 0, err
			}

			debug.Add("pricePercentChange", p)

			return p, nil
		},

		// stdev(start, end strDate)
		"stdev": func(args ...interface{}) (interface{}, error) {
			if len(args) < 2 {
				return 0, fmt.Errorf("stdev needs needed 2 args, got %d", len(args))
			}

			start, err := time.Parse(dateLayout, args[0].(string))
			if err != nil {
				return 0, err
			}
			end, err := time.Parse(dateLayout, args[1].(string))
			if err != nil {
				return 0, err
			}

			p, err := h.AnnualizedStdevOfDailyReturns(ctx, pr, symbol, start, end)
			if err != nil {
				return 0, err
			}
			debug.Add("stdev", p)

			return p, nil
		},
		"marketCap": func(args ...interface{}) (interface{}, error) {
			if len(args) < 1 {
				return 0, fmt.Errorf("marketCap needs needed 1 arg, got %d", len(args))
			}

			date, err := time.Parse(dateLayout, args[0].(string))
			if err != nil {
				return 0, err
			}

			return h.MarketCap(db, symbol, date)
		},
		"pbRatio": func(args ...interface{}) (interface{}, error) {
			if len(args) < 1 {
				return 0, fmt.Errorf("pbRatio needs needed 1 arg, got %d", len(args))
			}

			date, err := time.Parse(dateLayout, args[0].(string))
			if err != nil {
				return 0, err
			}

			return h.PbRatio(db, symbol, date)
		},
		"peRatio": func(args ...interface{}) (interface{}, error) {
			if len(args) < 1 {
				return 0, fmt.Errorf("peRatio needs needed 1 arg, got %d", len(args))
			}

			date, err := time.Parse(dateLayout, args[0].(string))
			if err != nil {
				return 0, err
			}

			return h.PeRatio(db, symbol, date)
		},
	}
}

type ExpressionResult struct {
	Value  float64
	Reason FormulaDebugger
}

func EvaluateFactorExpression(
	ctx context.Context,
	db *sql.DB,
	pr *l1_service.PriceCache,
	expression string,
	symbol string,
	factorMetricsHandler FactorMetricCalculations,
	date time.Time, // expressions are evaluated on the given date
) (*ExpressionResult, error) {
	eval := goval.NewEvaluator()
	variables := map[string]interface{}{
		"currentDate": date.Format(dateLayout),
	}

	debug := FormulaDebugger{}
	functions := constructFunctionMap(ctx, db, pr, symbol, factorMetricsHandler, debug, date)
	result, err := eval.Evaluate(expression, variables, functions)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate factor expression: %w", err)
	}

	// TODO - if it's a dry-run, we're not computing real results
	// and should allow any value to be processed here
	r, ok := result.(float64)
	if !ok {
		return nil, fmt.Errorf("failed to convert to float")
	} else if math.IsNaN(r) {
		return nil, fmt.Errorf("calculated NaN as expression result")
	} else if math.IsInf(r, 0) {
		return nil, fmt.Errorf("calculated infinity as expression result")
	}

	return &ExpressionResult{
		Value:  r,
		Reason: debug,
	}, nil
}

type FormulaDebugger map[string][]float64

func (f FormulaDebugger) Add(fieldName string, value float64) {
	if _, ok := f[fieldName]; !ok {
		f[fieldName] = make([]float64, 0)
	}
	f[fieldName] = append(f[fieldName], value)
}
