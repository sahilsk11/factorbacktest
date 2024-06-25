package internal

import (
	"database/sql"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/montanaflynn/stats"
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

func (b Bond) currentValue(t time.Time, interestRates domain.InterestRateMap) float64 {
	hoursTillExpiration := b.Expiration.Sub(t).Hours()
	monthsTillExpiration := int(hoursTillExpiration/720 + 0.005)
	if monthsTillExpiration <= 0 {
		return b.ParValue
	}

	marketRate, err := interestRates.GetRate(monthsTillExpiration)
	if err != nil {
		panic(fmt.Sprintf("no rate on %v", t))
	}
	remainingPayoutForMarket := b.ParValue * float64(monthsTillExpiration) / 12 * marketRate
	remainingPayoutForCurrent := b.ParValue * float64(monthsTillExpiration) / 12 * b.AnnualCouponRate

	return b.ParValue - (remainingPayoutForMarket - remainingPayoutForCurrent)
}

type Payment struct {
	Date   time.Time
	Amount float64
}

type BondPortfolio struct {
	Bonds                []Bond
	BondStreams          []map[uuid.UUID]struct{} // array of sets of IDS [{A, B, C}, {D, E, F}]
	Cash                 float64
	TargetDurationMonths []int
	CouponPayments       map[uuid.UUID][]Payment
}

func getBondStreamIndex(bondID uuid.UUID, streams []map[uuid.UUID]struct{}) int {
	for i := 0; i < len(streams); i++ {
		if _, ok := streams[i][bondID]; ok {
			return i
		}
	}
	panic(fmt.Sprintf("could not identify stream for %s", bondID.String()))
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
	streams := make([]map[uuid.UUID]struct{}, len(targetDurationMonths))
	for i, duration := range targetDurationMonths {
		amount := amountInvested / float64(len(targetDurationMonths))
		rate, err := interestRates.GetRate(duration)
		if err != nil {
			return nil, fmt.Errorf("failed to ger rate on date %v: %w", startDate, err)
		}

		bond := NewBond(amount, startDate, duration, rate)
		bonds = append(bonds, bond)
		streams[i] = map[uuid.UUID]struct{}{
			bond.ID: {},
		}
		remainingCash -= amount

	}

	return &BondPortfolio{
		Bonds:                bonds,
		Cash:                 remainingCash,
		TargetDurationMonths: targetDurationMonths,
		CouponPayments:       map[uuid.UUID][]Payment{},
		BondStreams:          streams,
	}, nil
}

func (bp BondPortfolio) calculateValue(t time.Time, interestRates domain.InterestRateMap) (float64, map[uuid.UUID]float64) {
	total := bp.Cash
	bondValues := map[uuid.UUID]float64{}
	for _, bond := range bp.Bonds {
		value := bond.currentValue(t, interestRates)
		bondValues[bond.ID] = value
		total += value
	}
	return total, bondValues
}

func (bp *BondPortfolio) refreshBondHoldings(today time.Time, interestRates domain.InterestRateMap) error {
	outBonds := []Bond{}

	for _, bond := range bp.Bonds {
		if today.Before(bond.Expiration) {
			outBonds = append(outBonds, bond)
		} else {
			value := bond.ParValue
			// buy a new bond with max duration
			duration := bp.TargetDurationMonths[len(bp.TargetDurationMonths)-1]
			newBondInceptionDate := bond.Expiration // buy the day the old expires
			streamID := getBondStreamIndex(bond.ID, bp.BondStreams)

			// TODO - consider cycling this bond if it happens to expire before today
			// which could happen with long duration

			rate, err := interestRates.GetRate(duration)
			if err != nil {
				return fmt.Errorf("failed to get rate on %v: %w", today, err)
			}

			newBond := NewBond(value, newBondInceptionDate, duration, rate)
			bp.BondStreams[streamID][newBond.ID] = struct{}{}
			outBonds = append(outBonds, newBond)
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
	err := bp.refreshBondHoldings(t, interestRates)
	if err != nil {
		return err
	}
	return nil
}

type CouponPaymentOnDate struct {
	BondPayments map[uuid.UUID]float64 `json:"bondPayments"`
	DateReceived time.Time             `json:"date"`
	// AverageCouponRate float64               `json:"averageCoupon"`
	DateStr     string  `json:"dateString"`
	TotalAmount float64 `json:"totalAmount"`
}

type BondPortfolioReturn struct {
	Date    time.Time `json:"date"`
	DateStr string    `json:"dateString"`

	ReturnSinceInception float64               `json:"returnSinceInception"`
	BondReturns          map[uuid.UUID]float64 `json:"bondReturns"`
}

type InterestRatesOnDate struct {
	Date           time.Time       `json:"date"`
	DateStr        string          `json:"dateString"`
	RateByDuration map[int]float64 `json:"rates"`
}

type Metrics struct {
	Stdev           float64 `json:"stdev"`
	AverageCoupon   float64 `json:"averageCoupon"`
	MaximumDrawdown float64 `json:"maxDrawdown"`
}

type BondLadderOnDate struct {
	Date                     time.Time `json:"date"`
	DateStr                  string    `json:"dateStr"`
	LadderTimeTillExpiration []float64 `json:"timeTillExpiration"`
	Unit                     string    `json:"unit"`
}

type BacktestBondPortfolioResult struct {
	CouponPayments  []CouponPaymentOnDate `json:"couponPayments"`
	PortfolioReturn []BondPortfolioReturn `json:"portfolioReturn"`
	InterestRates   []InterestRatesOnDate `json:"interestRates"`
	Metrics         Metrics               `json:"metrics"`
	BondStreams     [][]string            `json:"bondStreams"`
	BondLadder      []BondLadderOnDate    `json:"bondLadder"`
}

func (b BondService) BacktestBondPortfolio(
	tx *sql.Tx,
	durations []int,
	startingAmount float64,
	start time.Time,
	end time.Time,
) (*BacktestBondPortfolioResult, error) {
	current := start
	bp, err := b.ConstructBondPortfolio(tx, start, durations, startingAmount)
	if err != nil {
		return nil, err
	}

	// initialize total value and bond values

	portfolioReturns := []BondPortfolioReturn{}
	interestRatesOnDate := []InterestRatesOnDate{}
	bondLadderOnDates := []BondLadderOnDate{}

	dates := []time.Time{}
	for !current.After(end) {
		dates = append(dates, current)
		// make granularity one month
		current = current.AddDate(0, 1, 0)
	}
	interestRatesForBacktest, err := b.InterestRateRepository.GetRatesOnDates(dates, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to get interest rates: %w", err)
	}

	// current method likely misses last day
	// because granularity overshoots
	for _, date := range dates {
		dateStr := date.Format(time.DateOnly)
		interestRates, ok := interestRatesForBacktest[dateStr]
		if !ok {
			return nil, fmt.Errorf("missing %s from interest rates", dateStr)
		}

		// populate historic interest rates
		rateByDuration := map[int]float64{}
		for _, duration := range durations {
			rateByDuration[duration], err = interestRates.GetRate(duration)
			if err != nil {
				return nil, fmt.Errorf("failed to get rate for %s: %w", dateStr, err)
			}
		}
		interestRatesOnDate = append(interestRatesOnDate, InterestRatesOnDate{
			Date:           date,
			DateStr:        date.Format(time.DateOnly),
			RateByDuration: rateByDuration,
		})

		bp.Refresh(date, end, interestRates)

		totalValueOnDay, bondValuesOnDay := bp.calculateValue(date, interestRates)
		bondReturns := map[uuid.UUID]float64{}
		for _, bond := range bp.Bonds {
			startAmount := bond.ParValue
			currentValue := bondValuesOnDay[bond.ID]
			bondReturns[bond.ID] = (currentValue - startAmount) / startAmount
		}

		todayReturn := BondPortfolioReturn{
			Date:                 date,
			DateStr:              date.Format(time.DateOnly),
			ReturnSinceInception: (totalValueOnDay - startingAmount) / startingAmount,
			BondReturns:          bondReturns,
		}

		ladderTimes := make([]float64, len(bp.TargetDurationMonths))
		for _, bond := range bp.Bonds {
			timeUntilExpiration := bond.Expiration.Sub(date).Hours() / (24 * 365)
			streamIndex := getBondStreamIndex(bond.ID, bp.BondStreams)
			ladderTimes[streamIndex] = timeUntilExpiration
		}
		bondLadderOnDates = append(bondLadderOnDates, BondLadderOnDate{
			Date:                     date,
			DateStr:                  dateStr,
			LadderTimeTillExpiration: ladderTimes,
			Unit:                     "year",
		})

		portfolioReturns = append(portfolioReturns, todayReturn)
	}

	couponPayments, err := groupCouponPaymentsByDate(bp.CouponPayments)
	if err != nil {
		return nil, err
	}
	couponPayments = append([]CouponPaymentOnDate{
		{
			BondPayments: map[uuid.UUID]float64{},
			DateReceived: start,
			DateStr:      start.Format(time.DateOnly),
			TotalAmount:  0,
		},
	}, couponPayments...)

	metrics, err := computeMetrics(bp.Bonds, bp.CouponPayments, portfolioReturns)
	if err != nil {
		return nil, err
	}

	bondStreams := make([][]string, len(bp.BondStreams))
	for index, ids := range bp.BondStreams {
		bondStreams[index] = []string{}
		for id := range ids {
			bondStreams[index] = append(bondStreams[index], id.String())
		}
	}

	return &BacktestBondPortfolioResult{
		CouponPayments:  couponPayments,
		PortfolioReturn: portfolioReturns,
		InterestRates:   interestRatesOnDate,
		Metrics:         *metrics,
		BondStreams:     bondStreams,
		BondLadder:      bondLadderOnDates,
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
			DateStr:      dateStr,
			TotalAmount:  totalAmount,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].DateReceived.Before(out[j].DateReceived)
	})

	return out, nil
}

func computeMetrics(bonds []Bond, couponPayments map[uuid.UUID][]Payment, portfolioReturns []BondPortfolioReturn) (*Metrics, error) {
	// average coupon
	totalCouponPayment := 0.0
	totalBondParValues := 0.0
	for bondID, payments := range couponPayments {
		for _, bond := range bonds {
			if bond.ID == bondID {
				for _, p := range payments {
					totalCouponPayment += p.Amount * 12 // re-annualize values
					totalBondParValues += bond.ParValue
				}
			}
		}
	}

	stdev := 0.0
	maxDrawdown := 0.0
	var err error

	if len(portfolioReturns) > 1 {
		// standard deviation
		returns := []float64{portfolioReturns[0].ReturnSinceInception}
		for i := 1; i < len(portfolioReturns); i++ {
			prevReturn := returns[len(returns)-1]
			todayReturn := portfolioReturns[i].ReturnSinceInception
			returns = append(returns, todayReturn-prevReturn)
		}
		stdev, err = stats.StandardDeviationSample(returns)
		if err != nil {
			return nil, err
		}
		magicNumber := math.Sqrt(252)
		stdev *= magicNumber

		top := portfolioReturns[0].ReturnSinceInception
		for i := 1; i < len(portfolioReturns); i++ {
			todayReturn := portfolioReturns[i].ReturnSinceInception
			if todayReturn > top {
				top = todayReturn
			}
			drawdown := todayReturn - top
			if drawdown < maxDrawdown {
				maxDrawdown = drawdown
			}
		}
	}

	averageCoupon := 0.0
	if totalBondParValues > 0 {
		averageCoupon = totalCouponPayment / totalBondParValues
	}

	return &Metrics{
		AverageCoupon:   averageCoupon,
		Stdev:           stdev,
		MaximumDrawdown: maxDrawdown,
	}, nil
}
