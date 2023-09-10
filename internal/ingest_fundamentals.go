package internal

import (
	"alpha/internal/db/models/postgres/public/model"
	"alpha/internal/repository"
	"alpha/pkg/datajockey"
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"time"

	"github.com/go-jet/jet/v2/qrm"
)

func IngestUniverseFundamentals(
	db *sql.DB, // commit as we go for partial failures
	djClient datajockey.Client,
	afRepository repository.AssetFundamentalsRepository,
	universeRepository repository.UniverseRepository,
) error {
	assets, err := universeRepository.List(db)
	if err != nil {
		log.Fatal(err)
	}
	errors := []error{}
	for _, a := range assets {
		err = IngestFundamentals(
			db,
			djClient,
			a.Symbol,
			afRepository,
		)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to add %s: %w", a.Symbol, err))
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("failed to add %d asset data. errors: %v", len(errors), errors)
	}

	return nil
}

func IngestFundamentals(
	tx qrm.Executable,
	djClient datajockey.Client,
	symbol string,
	afRepository repository.AssetFundamentalsRepository,
) error {
	djResponse, err := djClient.GetAssetMetrics(symbol)
	if err != nil {
		return fmt.Errorf("failed to get dj asset metrics: %w", err)
	}
	models, err := invertDjResponse(symbol, djResponse.FinancialData.Quarterly)
	if err != nil {
		return err
	}
	fmt.Println(len(models))

	err = afRepository.Add(tx, models)
	if err != nil {
		return err
	}

	return nil
}

func extractYearAndQuarter(input string) (int, int, error) {
	// Define the regular expression pattern to capture the year and quarter
	pattern := `(\d{4})Q(\d)`

	// Compile the regular expression
	regExp, err := regexp.Compile(pattern)
	if err != nil {
		return 0, 0, err
	}

	// Find the matches in the input string
	matches := regExp.FindStringSubmatch(input)
	if len(matches) != 3 {
		return 0, 0, fmt.Errorf("match not found")
	}

	// Extract the year and quarter from the matches
	year, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, 0, err
	}

	quarter, err := strconv.Atoi(matches[2])
	if err != nil {
		return 0, 0, err
	}

	return year, quarter, nil
}

func mapQuarter(year, quarter int) (time.Time, time.Time) {
	quarterMap := map[int]struct {
		start time.Time
		end   time.Time
	}{
		1: {
			start: time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC),
			end:   time.Date(year, 4, 1, 0, 0, 0, 0, time.UTC),
		},
		2: {
			start: time.Date(year, 4, 1, 0, 0, 0, 0, time.UTC),
			end:   time.Date(year, 7, 1, 0, 0, 0, 0, time.UTC),
		},
		3: {
			start: time.Date(year, 7, 1, 0, 0, 0, 0, time.UTC),
			end:   time.Date(year, 10, 1, 0, 0, 0, 0, time.UTC),
		},
		4: {
			start: time.Date(year, 10, 1, 0, 0, 0, 0, time.UTC),
			end:   time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}
	x := quarterMap[quarter]

	return x.start, x.end
}

func invertDjResponse(asset string, in datajockey.Fields) ([]model.AssetFundamental, error) {
	mappedValues := map[string]*model.AssetFundamental{}

	{

		for k, v := range in.Revenue {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].Revenue = &x
		}

		for k, v := range in.CostOfRevenue {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].CostOfRevenue = &x
		}

		for k, v := range in.GrossProfit {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].GrossProfit = &x
		}

		for k, v := range in.OperatingIncome {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].OperatingIncome = &x
		}

		for k, v := range in.TotalAssets {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].TotalAssets = &x
		}

		for k, v := range in.TotalCurrentAssets {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].TotalCurrentAssets = &x
		}

		for k, v := range in.PrepaidExpenses {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].PrepaidExpenses = &x
		}

		for k, v := range in.PropertyPlantAndEquipmentNet {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].PropertyPlantAndEquipmentNet = &x
		}

		for k, v := range in.RetainedEarnings {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].RetainedEarnings = &x
		}

		for k, v := range in.OtherAssetsNoncurrent {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].OtherAssetsNoncurrent = &x
		}

		for k, v := range in.TotalNonCurrentAssets {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].TotalNonCurrentAssets = &x
		}

		for k, v := range in.TotalLiabilities {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].TotalLiabilities = &x
		}

		for k, v := range in.ShareholderEquity {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].ShareholderEquity = &x
		}

		for k, v := range in.NetIncome {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].NetIncome = &x
		}

		for k, v := range in.SharesOutstandingDiluted {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].SharesOutstandingDiluted = &x
		}

		for k, v := range in.SharesOutstandingBasic {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].SharesOutstandingBasic = &x
		}

		for k, v := range in.EpsDiluted {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].EpsDiluted = &x
		}

		for k, v := range in.EpsBasic {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].EpsBasic = &x
		}

		for k, v := range in.OperatingCashFlow {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].OperatingCashFlow = &x
		}

		for k, v := range in.InvestingCashFlow {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].InvestingCashFlow = &x
		}

		for k, v := range in.FinancingCashFlow {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].FinancingCashFlow = &x
		}

		for k, v := range in.NetCashFlow {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].NetCashFlow = &x
		}

		for k, v := range in.ResearchDevelopmentExpense {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].ResearchDevelopmentExpense = &x
		}

		for k, v := range in.SellingGeneralAdministrativeExpense {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].SellingGeneralAdministrativeExpense = &x
		}

		for k, v := range in.OperatingExpenses {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].OperatingExpenses = &x
		}

		for k, v := range in.NonOperatingIncome {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].NonOperatingIncome = &x
		}

		for k, v := range in.PreTaxIncome {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].PreTaxIncome = &x
		}

		for k, v := range in.IncomeTax {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].IncomeTax = &x
		}

		for k, v := range in.DepreciationAmortization {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].DepreciationAmortization = &x
		}

		for k, v := range in.StockBasedCompensation {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].StockBasedCompensation = &x
		}

		for k, v := range in.DividendsPaid {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].DividendsPaid = &x
		}

		for k, v := range in.CashOnHand {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].CashOnHand = &x
		}

		for k, v := range in.CurrentNetReceivables {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].CurrentNetReceivables = &x
		}

		for k, v := range in.Inventory {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].Inventory = &x
		}

		for k, v := range in.TotalCurrentLiabilities {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].TotalCurrentLiabilities = &x
		}

		for k, v := range in.TotalNonCurrentLiabilities {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].TotalNonCurrentLiabilities = &x
		}

		for k, v := range in.LongTermDebt {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].LongTermDebt = &x
		}

		for k, v := range in.TotalLongTermLiabilities {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].TotalLongTermLiabilities = &x
		}

		for k, v := range in.Goodwill {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].Goodwill = &x
		}

		for k, v := range in.IntangibleAssetsExcludingGoodwill {
			if _, ok := mappedValues[k]; !ok {
				mappedValues[k] = &model.AssetFundamental{}
			}
			x := float64(v)
			mappedValues[k].IntangibleAssetsExcludingGoodwill = &x
		}

	}
	out := []model.AssetFundamental{}

	for k, v := range mappedValues {
		year, quarter, err := extractYearAndQuarter(k)
		if err != nil {
			return nil, err
		}
		start, end := mapQuarter(year, quarter)
		now := time.Now()
		out = append(out, model.AssetFundamental{
			Symbol:                              asset,
			Granularity:                         model.AssetFundamentalGranularity_Quarterly,
			StartDate:                           start,
			EndDate:                             end,
			CreatedAt:                           &now,
			Revenue:                             v.Revenue,
			CostOfRevenue:                       v.CostOfRevenue,
			GrossProfit:                         v.GrossProfit,
			OperatingIncome:                     v.OperatingIncome,
			TotalAssets:                         v.TotalAssets,
			TotalCurrentAssets:                  v.TotalCurrentAssets,
			PrepaidExpenses:                     v.PrepaidExpenses,
			PropertyPlantAndEquipmentNet:        v.PropertyPlantAndEquipmentNet,
			RetainedEarnings:                    v.RetainedEarnings,
			OtherAssetsNoncurrent:               v.OtherAssetsNoncurrent,
			TotalNonCurrentAssets:               v.TotalNonCurrentAssets,
			TotalLiabilities:                    v.TotalLiabilities,
			ShareholderEquity:                   v.ShareholderEquity,
			NetIncome:                           v.NetIncome,
			SharesOutstandingDiluted:            v.SharesOutstandingDiluted,
			SharesOutstandingBasic:              v.SharesOutstandingBasic,
			EpsDiluted:                          v.EpsDiluted,
			EpsBasic:                            v.EpsBasic,
			OperatingCashFlow:                   v.OperatingCashFlow,
			InvestingCashFlow:                   v.InvestingCashFlow,
			FinancingCashFlow:                   v.FinancingCashFlow,
			NetCashFlow:                         v.NetCashFlow,
			ResearchDevelopmentExpense:          v.ResearchDevelopmentExpense,
			SellingGeneralAdministrativeExpense: v.SellingGeneralAdministrativeExpense,
			OperatingExpenses:                   v.OperatingExpenses,
			NonOperatingIncome:                  v.NonOperatingIncome,
			PreTaxIncome:                        v.PreTaxIncome,
			IncomeTax:                           v.IncomeTax,
			DepreciationAmortization:            v.DepreciationAmortization,
			StockBasedCompensation:              v.StockBasedCompensation,
			DividendsPaid:                       v.DividendsPaid,
			CashOnHand:                          v.CashOnHand,
			CurrentNetReceivables:               v.CurrentNetReceivables,
			Inventory:                           v.Inventory,
			TotalCurrentLiabilities:             v.TotalCurrentLiabilities,
			TotalNonCurrentLiabilities:          v.TotalNonCurrentLiabilities,
			LongTermDebt:                        v.LongTermDebt,
			TotalLongTermLiabilities:            v.TotalLongTermLiabilities,
			Goodwill:                            v.Goodwill,
			IntangibleAssetsExcludingGoodwill:   v.IntangibleAssetsExcludingGoodwill,
		})
	}

	return out, nil
}
