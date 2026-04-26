package progress

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSSEReporter_WireFormat locks down the on-the-wire contract that the
// frontend depends on: one `data: {...}\n\n` SSE frame per Step call,
// step_started carrying id+label, step_completed carrying id+durationMs.
// If this test breaks, the FE breaks. Everything else in this package
// (noop default, context plumbing, Reporter interface) is either trivial
// glue the type system enforces or covered transitively by this test.
func TestSSEReporter_WireFormat(t *testing.T) {
	rec := httptest.NewRecorder()
	w, err := NewSSEWriter(rec)
	require.NoError(t, err)

	done := NewSSEReporter(w).Step("factor_scores", "Calculating factor scores")
	done()

	var frames []Event
	for _, line := range strings.Split(rec.Body.String(), "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var ev Event
		require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev))
		frames = append(frames, ev)
	}

	require.Len(t, frames, 2)
	require.Equal(t, Event{Type: "step_started", Step: "factor_scores", Label: "Calculating factor scores"}, frames[0])
	require.Equal(t, "step_completed", frames[1].Type)
	require.Equal(t, "factor_scores", frames[1].Step)
	require.GreaterOrEqual(t, frames[1].DurationMs, int64(0))
}
