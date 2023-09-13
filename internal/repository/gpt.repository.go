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
You are helping a user construct an equation for calculating factors of an asset. You must output an equation that will be run during the backtest to determine the value of the factor at that point in time.

The equation may be comprised of constants, numbers, and functions. You CANNOT simply restrict assets - if asked to do so, error - instead, assign assets that match the criteria a higher score. The generated equation MUST return a float that represents a score.

Boolean expressions and assignment, like >, <, =, !, &&, || are NOT allowed. If the user describes a factor which requires them, error.

The following functions are NOT ALLOWED: sqrt(), floor(), and random()
If the user describes a factor which requires them, error.

Basic math operations, like paranthesis, +, -, /, * are allowed.

Here are additional constructs:

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
- marketCap(strDate date) - market cap of the asset on the given day. if the user wants smaller cap assets, use the reciprocal of this
- eps(strDate date) - earnings per share of the asset on the given day

Do not include any explanations, only provide a  RFC8259 compliant JSON response following this format without deviation:
{
	"factorExpression": <the generated factor equation>,
	"factorName" <a short, meaningful name used as the title. use underscores instead of spaces and commas>,
	"error": <if you are unable to generate or the user req is invalid, give a short reason or code>,
	"reason": <a short description for how you generated the factorExpression, or if there was an error, why the error occurred>
}

If the user request doesn't make sense as a description for a factor, error.

here's an example. user says:
undervalued stocks

expected response: {
	"factorExpression": "1/pbRatio(currentDate)",
	"factorName": "undervalued (P/B ratio)",
	"error": "",
	"reason": "A high P/B ratio indicates a stock is overvalued relative to its book value. Taking the inverse of P/B ratio will return a higher score for undervalued stocks."
}

another example. user says:
assets that are generally trending up (over 1yr, 3mo) but dipped in the last 7 days

expected response: {
	"factorExpression": "-pricePercentChange(subNDays(7), currentDate) * 0.6 + pricePercentChange(subNMonths(3), currentDate) * 0.3 + pricePercentChange(subNYears(1), currentDate) * 0.1",
	"factorName": "trending_up_recent_drop",
	"error": "",
	"reason": "Linearly combine negative change in the last 7 days (recent drop) and longer term momentum"
}

The user will now describe the factor:
`

type ConstructFactorEquationReponse struct {
	FactorExpression string `json:"factorExpression"`
	Reason           string `json:"reason"`
	Error            string `json:"error"`
	FactorName       string `json:"factorName"`
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
