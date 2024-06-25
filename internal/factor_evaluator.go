package internal

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/maja42/goval"
)

const dateLayout = "2006-01-02"

func constructFunctionMap(
	tx *sql.Tx,
	symbol string,
	h FactorMetricCalculations,
	debug FormulaDebugger,
) map[string]goval.ExpressionFunction {
	return map[string]goval.ExpressionFunction{
		// we could break this up

		// helper functions
		"addDate": func(args ...interface{}) (interface{}, error) {
			// addDate(date, years, months, days)
			date, err := time.Parse(dateLayout, args[0].(string))
			if err != nil {
				return 0, err
			}

			var years, months, days = args[1].(int), args[2].(int), args[3].(int)

			date = date.AddDate(years, months, days)

			return date.Format(dateLayout), nil
		},

		// metric functions
		"price": func(args ...interface{}) (interface{}, error) {
			// price(date strDate)
			date, err := time.Parse(dateLayout, args[0].(string))
			if err != nil {
				return 0, err
			}
			p, err := h.Price(tx, symbol, date)
			if err != nil {
				return 0, err
			}

			debug.Add("price", p)

			return p, nil
		},
		"pricePercentChange": func(args ...interface{}) (interface{}, error) {
			// pricePercentChange(start, end strDate)
			start, err := time.Parse(dateLayout, args[0].(string))
			if err != nil {
				return 0, err
			}
			end, err := time.Parse(dateLayout, args[1].(string))
			if err != nil {
				return 0, err
			}

			p, err := h.PricePercentChange(tx, symbol, start, end)
			if err != nil {
				return 0, err
			}

			debug.Add("pricePercentChange", p)

			return p, nil
		},
		"stdev": func(args ...interface{}) (interface{}, error) {
			// stdev(start, end strDate)
			start, err := time.Parse(dateLayout, args[0].(string))
			if err != nil {
				return 0, err
			}
			end, err := time.Parse(dateLayout, args[1].(string))
			if err != nil {
				return 0, err
			}

			p, err := h.AnnualizedStdevOfDailyReturns(tx, symbol, start, end)
			if err != nil {
				return 0, err
			}
			debug.Add("stdev", p)

			return p, nil
		},
		"marketCap": func(args ...interface{}) (interface{}, error) {
			date, err := time.Parse(dateLayout, args[0].(string))
			if err != nil {
				return 0, err
			}

			return h.MarketCap(tx, symbol, date)
		},
		"pbRatio": func(args ...interface{}) (interface{}, error) {
			date, err := time.Parse(dateLayout, args[0].(string))
			if err != nil {
				return 0, err
			}

			return h.PbRatio(tx, symbol, date)
		},
		"peRatio": func(args ...interface{}) (interface{}, error) {
			date, err := time.Parse(dateLayout, args[0].(string))
			if err != nil {
				return 0, err
			}

			return h.PeRatio(tx, symbol, date)
		},
	}
}

type ExpressionResult struct {
	Value  float64
	Reason FormulaDebugger
}

func EvaluateFactorExpression(
	tx *sql.Tx,
	expression string,
	symbol string,
	factorMetricsHandler FactorMetricCalculations,
	date time.Time, // expressions are evaluated on the given date
) (*ExpressionResult, error) {
	// Implementing strlen()
	eval := goval.NewEvaluator()
	variables := map[string]interface{}{
		"currentDate": date.Format(dateLayout),
	}

	debug := FormulaDebugger{}
	functions := constructFunctionMap(tx, symbol, factorMetricsHandler, debug)
	result, err := eval.Evaluate(expression, variables, functions)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate factor expression: %w", err)
	}

	r, ok := result.(float64)
	if !ok {
		return nil, fmt.Errorf("failed to convert to float")
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
