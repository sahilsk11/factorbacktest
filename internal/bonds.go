package internal

import (
	"database/sql"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	"sort"
	"time"

	"github.com/google/uuid"
)

type Bond struct {
	ID               uuid.UUID
	Creation         time.Time
	Expiration       time.Time
	AnnualCouponRate float64
	ParValue         float64
}

func NewBond(amount float64, creationDate time.Time, durationMonths int, rate float64) Bond {
	expiration := creationDate.AddDate(0, durationMonths, 0)

	return Bond{
		ID:               uuid.New(),
		Creation:         creationDate,
		Expiration:       expiration,
		AnnualCouponRate: rate,
		ParValue:         amount,
	}
}

type BondService struct {
	InterestRateRepository repository.InterestRateRepository
}

// TODO - fix this jank function
func (b Bond) currentValue(t time.Time, interestRates domain.InterestRateMap) float64 {
	hoursTillExpiration := b.Expiration.Sub(t).Hours()
	monthsTillExpiration := int(hoursTillExpiration/730 + 0.005)
	if monthsTillExpiration <= 0 {
		return b.ParValue
	}

	marketRate := interestRates.GetRate(monthsTillExpiration)

	// totally wrong, but somewhat accounts for bond price dropping
	// if marketRate
	out := b.ParValue * (1 + (b.AnnualCouponRate-marketRate)*2/float64(monthsTillExpiration))

	return out
}

type Payment struct {
	Date   time.Time
	Amount float64
}

type BondPortfolio struct {
	Bonds                []Bond
	Cash                 float64
	TargetDurationMonths []int
	CouponPayments       map[uuid.UUID][]Payment
}

func (b BondService) ConstructBondPortfolio(
	tx *sql.Tx,
	startDate time.Time,
	targetDurationMonths []int,
	amountInvested float64,
) (*BondPortfolio, error) {
	interestRates, err := b.InterestRateRepository.GetRatesOnDate(startDate, tx)
	if err != nil {
		return nil, err
	}
	bonds := []Bond{}
	remainingCash := amountInvested
	for _, duration := range targetDurationMonths {
		amount := amountInvested / float64(len(targetDurationMonths))
		rate := interestRates.GetRate(duration)
		bonds = append(bonds, NewBond(amount, startDate, duration, rate))
		remainingCash -= amount
	}

	return &BondPortfolio{
		Bonds:                bonds,
		Cash:                 remainingCash,
		TargetDurationMonths: targetDurationMonths,
		CouponPayments:       map[uuid.UUID][]Payment{},
	}, nil
}

func (bp BondPortfolio) calculateValue(t time.Time, interestRates domain.InterestRateMap) (float64, map[uuid.UUID]float64) {
	// temporarily not including cash, since we only care
	// about bond values
	total := 0.0
	bondValues := map[uuid.UUID]float64{}
	for _, bond := range bp.Bonds {
		value := bond.currentValue(t, interestRates)
		bondValues[bond.ID] = value
		total += value
	}
	return total, bondValues
}

func (bp *BondPortfolio) refreshBondHoldings(today time.Time, interestRates domain.InterestRateMap, backtestEnd time.Time) error {
	outBonds := []Bond{}

	for _, bond := range bp.Bonds {
		if today.Before(bond.Expiration) {
			outBonds = append(outBonds, bond)
		} else {
			value := bond.ParValue
			// buy a new bond with max duration
			// and value of exited bond
			duration := bp.TargetDurationMonths[len(bp.TargetDurationMonths)-1]
			newBondInceptionDate := bond.Expiration // buy the day the old expires

			// if the bond will expire before the end of the backtest, include it
			// there's some weird edge cases as we approach end of the backtest,
			// like should we buy smaller duration?
			// TODO - consider cycling this bond if it happens to expire before today
			// which could happen with long duration
			expiration := newBondInceptionDate.AddDate(0, duration, 0)
			if !expiration.After(backtestEnd) {
				rate := interestRates.GetRate(duration)
				outBonds = append(outBonds, NewBond(value, bond.Expiration, duration, rate))
			} else {
				bp.Cash += value
			}
		}
	}
	sort.Slice(outBonds, func(i, j int) bool {
		return outBonds[i].Expiration.Before(outBonds[j].Expiration)
	})
	bp.Bonds = outBonds

	return nil
}

// TODO - this needs to figure out all missing payments
// from the last payment, and add them on the correct date
// will allow us to decouple payment logic from granularity
func (bp *BondPortfolio) refreshCouponPayments(t time.Time) {
	for _, bond := range bp.Bonds {
		paymentAmount := bond.ParValue * bond.AnnualCouponRate / 12
		if _, ok := bp.CouponPayments[bond.ID]; !ok {
			bp.CouponPayments[bond.ID] = []Payment{}
		}

		// if no payments and current time is >= 1 month from bond inception
		addFirstPayment := len(bp.CouponPayments[bond.ID]) == 0 && !t.Before(bond.Creation.AddDate(0, 1, 0))
		if addFirstPayment {
			bp.CouponPayments[bond.ID] = append(bp.CouponPayments[bond.ID], Payment{
				Date:   bond.Creation.AddDate(0, 1, 0),
				Amount: paymentAmount,
			})
		}

		morePayments := true

		for len(bp.CouponPayments[bond.ID]) > 0 && morePayments {
			payments := bp.CouponPayments[bond.ID]
			lastPayment := payments[len(payments)-1]
			// if the most recent payment was > 30 days ago, add a payment
			if !t.Before(lastPayment.Date.AddDate(0, 1, 0)) {
				bp.CouponPayments[bond.ID] = append(bp.CouponPayments[bond.ID], Payment{
					Date:   lastPayment.Date.AddDate(0, 1, 0),
					Amount: paymentAmount,
				})
			} else {
				morePayments = false
			}
		}
	}
}

func (bp *BondPortfolio) Refresh(t time.Time, backtestEnd time.Time, interestRates domain.InterestRateMap) error {
	bp.refreshCouponPayments(t)
	err := bp.refreshBondHoldings(t, interestRates, backtestEnd)
	if err != nil {
		return err
	}
	return nil
}

/*

then for performance

[
	{
		day
		totalReturnFromInception
		bonds: [{
			id
			returns: [{
				date,
				value // make sure this starts from prev ret of chain
			}]
		}]
	}
]

*/

type CouponPaymentOnDate struct {
	BondPayments map[uuid.UUID]float64 `json:"bondPayments"`
	DateReceived time.Time             `json:"dateReceived"`
	TotalAmount  float64               `json:"totalAmount"`
}

type BondPortfolioReturn struct {
	Date        time.Time             `json:"date"`
	TotalReturn float64               `json:"totalReturn"`
	BondReturns map[uuid.UUID]float64 `json:"bondReturns"`
}

type BacktestBondPortfolioResult struct {
	CouponPayments []CouponPaymentOnDate `json:"couponPayments"`
	Returns        []BondPortfolioReturn `json:"returns"`
}

type ValueOnDay struct {
	Value float64
	Date  time.Time
}

func (b BondService) BacktestBondPortfolio(
	tx *sql.Tx,
	durations []int,
	startingAmount float64,
	start time.Time,
	end time.Time,
) (*BacktestBondPortfolioResult, error) {
	granularityDays := 15

	current := start
	bp, err := b.ConstructBondPortfolio(tx, start, durations, startingAmount)
	if err != nil {
		return nil, err
	}

	// initialize total value and bond values

	bondValues := map[uuid.UUID][]ValueOnDay{}
	for _, bond := range bp.Bonds {
		bondValues[bond.ID] = []ValueOnDay{{
			Date:  start,
			Value: bond.ParValue,
		}}
	}

	returns := []BondPortfolioReturn{}

	current = current.AddDate(0, 0, granularityDays)

	// current method likely misses last day
	// because granularity overshoots
	for !current.After(end) {
		interestRates, err := b.InterestRateRepository.GetRatesOnDate(current, tx)
		if err != nil {
			return nil, err
		}

		// before dumping the current bonds, i'd like to show
		// that their return goes back to 0. in other words, when we
		// dump a bond, somehow add one more entry to the date. maybe we can
		// do this afterwards?
		// like, for each bond return stream, add a 1 at the end
		bp.Refresh(current, end, *interestRates)

		totalValueOnDay, bondValuesOnDay := bp.calculateValue(current, *interestRates)
		// I don't know if this is always true
		// TODO - cleanup
		bondReturns := map[uuid.UUID]float64{}
		for id, value := range bondValuesOnDay {
			for _, bond := range bp.Bonds {
				if bond.ID == id {
					bondReturns[id] = (value - bond.ParValue) / bond.ParValue
				}
			}
		}

		returns = append(returns, BondPortfolioReturn{
			Date:        current,
			TotalReturn: (totalValueOnDay - startingAmount) / startingAmount,
			BondReturns: bondReturns,
		})

		current = current.AddDate(0, 0, granularityDays)
	}

	couponPayments, err := groupCouponPaymentsByDate(bp.CouponPayments)
	if err != nil {
		return nil, err
	}

	return &BacktestBondPortfolioResult{
		CouponPayments: couponPayments,
		Returns:        returns,
	}, nil
}

func groupCouponPaymentsByDate(couponPayments map[uuid.UUID][]Payment) ([]CouponPaymentOnDate, error) {
	paymentsOnDate := map[string]map[uuid.UUID]float64{}
	for bondID, payments := range couponPayments {
		for _, payment := range payments {
			dateStr := payment.Date.Format(time.DateOnly)
			if _, ok := paymentsOnDate[dateStr]; !ok {
				paymentsOnDate[dateStr] = map[uuid.UUID]float64{}
			}
			paymentsOnDate[dateStr][bondID] = payment.Amount
		}
	}
	out := []CouponPaymentOnDate{}
	for dateStr, bondPayments := range paymentsOnDate {
		date, err := time.Parse(time.DateOnly, dateStr)
		if err != nil {
			return nil, err
		}
		totalAmount := 0.0
		for _, payment := range bondPayments {
			totalAmount += payment
		}

		out = append(out, CouponPaymentOnDate{
			BondPayments: bondPayments,
			DateReceived: date,
			TotalAmount:  totalAmount,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].DateReceived.Before(out[j].DateReceived)
	})

	return out, nil
}
