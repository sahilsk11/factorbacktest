package internal

import (
	interestrate "factorbacktest/pkg/interest_rate"
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

func (b Bond) currentValue(t time.Time, interestRates interestrate.InterestRateMap) float64 {
	if t.After(b.Expiration) {
		return b.ParValue
	}
	hoursTillExpiration := b.Expiration.Sub(t).Hours()
	monthsTillExpiration := int(hoursTillExpiration/730 + 0.005)
	marketRate := interestRates.GetRate(monthsTillExpiration)

	// totally wrong, but somewhat accounts for bond price dropping
	// if marketRate
	return b.ParValue * (1 + (b.AnnualCouponRate-marketRate)*2/float64(monthsTillExpiration))
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

func ConstructBondPortfolio(
	startDate time.Time,
	targetDurationMonths []int,
	amountInvested float64,
) (*BondPortfolio, error) {
	interestRates, err := interestrate.GetYieldCurve(startDate)
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

func (bp *BondPortfolio) refreshBondHoldings(t time.Time, interestRates interestrate.InterestRateMap) error {
	outBonds := []Bond{}

	for _, bond := range bp.Bonds {
		if t.Before(bond.Expiration) {
			outBonds = append(outBonds, bond)
		} else {
			value := bond.ParValue
			// buy a new bond with max duration
			// and value of exited bond
			duration := bp.TargetDurationMonths[len(bp.TargetDurationMonths)-1]
			rate := interestRates.GetRate(duration)
			outBonds = append(outBonds, NewBond(value, t, duration, rate))
		}
	}
	sort.Slice(outBonds, func(i, j int) bool {
		return outBonds[i].Expiration.Before(outBonds[j].Expiration)
	})
	bp.Bonds = outBonds

	return nil
}

func (bp *BondPortfolio) refreshCouponPayments(t time.Time) {
	for _, bond := range bp.Bonds {
		paymentAmount := bond.ParValue * bond.AnnualCouponRate / 12
		if _, ok := bp.CouponPayments[bond.ID]; !ok {
			bp.CouponPayments[bond.ID] = []Payment{}
		}

		// if no values, add if 30 days from start
		firstPayment := len(bp.CouponPayments[bond.ID]) == 0 && t.Sub(bond.Creation).Hours() >= 730
		followUpPayment := false
		if len(bp.CouponPayments[bond.ID]) > 0 {
			payments := bp.CouponPayments[bond.ID]
			lastPayment := payments[len(payments)-1]
			if t.Sub(lastPayment.Date).Hours() >= 730 {
				followUpPayment = true
			}
		}

		if firstPayment || followUpPayment {
			bp.CouponPayments[bond.ID] = append(bp.CouponPayments[bond.ID], Payment{
				Date:   t,
				Amount: paymentAmount,
			})
			bp.Cash += paymentAmount
		}
	}
}

func (bp *BondPortfolio) Refresh(t time.Time) error {
	interestRates, err := interestrate.GetYieldCurve(t)
	if err != nil {
		return err
	}
	bp.refreshCouponPayments(t)
	err = bp.refreshBondHoldings(t, *interestRates)
	if err != nil {
		return err
	}
	return nil
}
