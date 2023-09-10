package repository

import (
	"fmt"

	"github.com/ayush6624/go-chatgpt"
)

type GptRepository interface {
	ConstructFactorEquation(description string) (string, error)
}

type gptRepositoryHandler struct {
	GptClient *chatgpt.Client
}

func NewGptRepository(apiKey string) (GptRepository, error) {
	client, err := chatgpt.NewClient(apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to construct gpt client: %w", err)
	}

	return gptRepositoryHandler{
		GptClient: client,
	}, nil
}

const prompt = `
You are helping a user construct an equation for calculating factors of an asset. They will describe in English how the factor should be calculated. You must output an equation that will be run during the backtest to determine the value of the factor at that point in time.

The equation may be comprised of constants, numbers, and functions - basically any "regular" math operations. Here are the constructs:

types:
- strDate = date as a string formatted as "YYYY-MM-DD"

constants:
- currentDate = a constant of type strDate that represents the date that the calculation is occurring

functions:
- nDaysAgo(int days) - subtracts n days from currentDate
- nMonthsAgo(int months) - subtracts n months from currentDate
- nYearsAgo(int years) - subtracts n years from currentDate
note for all these functions, negative numbers are not allowed, as that would look into the future which is not possible in a backtest
- price(symbol string, strDate date) - retrieves the price of the ticker on a given day
- pricePercentChange(strDate start, strDate end) - percent change of the asset's price
- stdev(strDate start, strDate end) - the annualized standard deviation of daily returns over the given period
- pbRatio(symbol string, date)
- peRatio(symbol string, date)
- marketCap(symbol string, date)

here's an example:
the factor should be calculated by averaging the last 6 month, 12 month, and 18 month returns, then dividing that by the 3 year standard deviation:

expected output:
(
  (
    pricePercentChange(
      addDate(currentDate, 0, -6, 0),
      currentDate
    ) + pricePercentChange(
      addDate(currentDate, 0, -12, 0),
      currentDate
    ) + pricePercentChange(
      addDate(currentDate, 0, -18, 0),
      currentDate
    )
  ) / 3
) / stdev(addDate(currentDate, -3, 0, 0), currentDate)

`

func (h gptRepositoryHandler) ConstructFactorEquation(description string) (string, error) {
	return "", nil
}
