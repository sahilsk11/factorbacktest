package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_sortUniverses(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		input := []getAssetUniversesResponse{

			{
				Code: "SPY_TOP_100",
			},
			{
				Code: "SPY_TOP_80",
			},
		}
		sortUniverses(input)

		require.Equal(t, input[0].Code, "SPY_TOP_80")
	})
}
