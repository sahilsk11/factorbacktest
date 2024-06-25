package internal

import (
	interestrate "factorbacktest/pkg/interest_rate"
	"time"
)

type Bond struct {
	Expiration       time.Time
	AnnualCouponRate float64
	ParValue         float64
}

func (b Bond) currentValue(interestRates interestrate.InterestRateMap) float64 {
	hoursTillExpiration := b.Expiration.Sub(time.Now()).Hours()
	monthsTillExpiration := int(hoursTillExpiration/730 + 0.005)
	marketRate := interestRates.GetRate(monthsTillExpiration)

	// totally wrong, but somewhat accounts for bond price dropping
	// if marketRate
	return b.ParValue * (1 + (b.AnnualCouponRate-marketRate)*2/float64(monthsTillExpiration))
}

type BondPortfolio struct {
	Bonds                []Bond
	Cash                 float64
	TargetDurationMonths []int
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
		rate := interestRates.GetRate(duration)
		amount := amountInvested / float64(len(targetDurationMonths))
		expiration := startDate.AddDate(0, duration, 0)
		bonds = append(bonds, Bond{
			Expiration:       expiration,
			AnnualCouponRate: rate,
			ParValue:         amount,
		})
		remainingCash -= amount
	}

	return &BondPortfolio{
		Bonds:                bonds,
		Cash:                 remainingCash,
		TargetDurationMonths: targetDurationMonths,
	}, nil
}

func (bp *BondPortfolio) CheckAndRefreshHoldings(t time.Time) error {
	interestRates, err := interestrate.GetYieldCurve(t)
	if err != nil {
		return err
	}

	outBonds := []Bond{}

	for _, bond := range bp.Bonds {
		if time.Now().Before(bond.Expiration) {
			outBonds = append(outBonds, bond)
		} else {
			value := bond.currentValue(*interestRates)
			// buy a new bond with max duration
			// and value of exited bond
			duration := bp.TargetDurationMonths[len(bp.TargetDurationMonths)-1]
			rate := interestRates.GetRate(duration)
			expiration := t.AddDate(0, duration, 0)
			outBonds = append(outBonds, Bond{
				Expiration:       expiration,
				AnnualCouponRate: rate,
				ParValue:         value,
			})
		}
	}
	bp.Bonds = outBonds

	return nil
}
