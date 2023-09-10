package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ayush6624/go-chatgpt"
)

type GptRepository interface {
	ConstructFactorEquation(ctx context.Context, description string) (*ConstructFactorEquationReponse, error)
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
- price(strDate date) - retrieves the price of the ticker on a given day
- pricePercentChange(strDate start, strDate end) - percent change of the asset's price
- stdev(strDate start, strDate end) - the annualized standard deviation of daily returns over the given period
- pbRatio(strDate date) - price-to-book ratio of the asset on the given day
- peRatio(strDate date) - price-to-book ratio of the asset on the given day
- marketCap(strDate date) - market cap of the asset on the given day
- eps(strDate date) - earnings per share of the asset on the given day

here's an example. user says:
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

Respond ONLY IN THE FOLLOWING JSON FORMAT:
{
	"factorExpression": <the generated factor equation>,
	"error": <if you are unable to generate or the user req is invalid, give a short reason or code>,
	"reason": <a short description for how you generated the factorExpression, or if there was an error, why the error occurred>
}
`

type ConstructFactorEquationReponse struct {
	FactorExpression string `json:"factorExpression"`
	Reason           string `json:"reason"`
	Error            string `json:"error"`
}

func (h gptRepositoryHandler) ConstructFactorEquation(ctx context.Context, description string) (*ConstructFactorEquationReponse, error) {
	response, err := h.GptClient.Send(ctx, &chatgpt.ChatCompletionRequest{
		Model: chatgpt.GPT35Turbo,
		Messages: []chatgpt.ChatMessage{
			{
				Role:    chatgpt.ChatGPTModelRoleSystem,
				Content: prompt,
			},
			{
				Role:    chatgpt.ChatGPTModelRoleUser,
				Content: description,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to construct GPT response: %w", err)
	}

	out := ConstructFactorEquationReponse{}
	err = json.Unmarshal([]byte(response.Choices[0].Message.Content), &out)
	if err != nil {
		return nil, fmt.Errorf("failed to read gpt output: %w", err)
	}

	return &out, nil
}
