package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSamplingIntervalDuration(t *testing.T) {
	require.Equal(t, 24*time.Hour, samplingIntervalDuration("daily"))
	require.Equal(t, 7*24*time.Hour, samplingIntervalDuration("weekly"))
	require.Equal(t, 30*24*time.Hour, samplingIntervalDuration("monthly"))
	require.Equal(t, 365*24*time.Hour, samplingIntervalDuration("yearly"))
	require.Equal(t, 24*time.Hour, samplingIntervalDuration(""))
}

func TestPublishedBacktestWindow(t *testing.T) {
	now := time.Date(2026, 9, 28, 15, 4, 5, 0, time.UTC)
	start, end := publishedBacktestWindow(now)
	require.Equal(t, time.Date(2023, 9, 28, 0, 0, 0, 0, time.UTC), start)
	require.Equal(t, time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC), end)
}

func TestMarshalUnmarshalBacktestResult(t *testing.T) {
	got, err := unmarshalBacktestResult(nil)
	require.NoError(t, err)
	require.Nil(t, got)

	empty := ""
	got, err = unmarshalBacktestResult(&empty)
	require.NoError(t, err)
	require.Nil(t, got)

	raw, err := marshalBacktestResult(&BacktestResponse{FactorName: "Quality"})
	require.NoError(t, err)
	require.NotNil(t, raw)

	got, err = unmarshalBacktestResult(raw)
	require.NoError(t, err)
	require.Equal(t, "Quality", got.FactorName)
}
