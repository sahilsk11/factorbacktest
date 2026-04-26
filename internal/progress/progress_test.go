package progress

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNoopReporter_DoesNothing(t *testing.T) {
	r := Noop()
	done := r.Step("x", "y")
	done()
	done() // calling twice is a no-op (safe)
}

func TestFromContext_DefaultsToNoop(t *testing.T) {
	r := FromContext(context.Background())
	require.NotNil(t, r)
	// Verify we can call Step without panicking.
	r.Step("x", "y")()
}

func TestFromContext_RoundTripsReporter(t *testing.T) {
	custom := &fakeReporter{}
	ctx := WithReporter(context.Background(), custom)
	require.Same(t, custom, FromContext(ctx))
}

func TestStepHelper_RoutesThroughContext(t *testing.T) {
	custom := &fakeReporter{}
	ctx := WithReporter(context.Background(), custom)
	done := Step(ctx, "id1", "label1")
	done()
	require.Equal(t, []string{"start:id1:label1", "done:id1"}, custom.calls)
}

func TestSSEReporter_WritesStartAndCompleteFrames(t *testing.T) {
	rec := httptest.NewRecorder()
	// httptest.ResponseRecorder implements http.Flusher.
	w, err := NewSSEWriter(rec)
	require.NoError(t, err)
	r := NewSSEReporter(w)

	done := r.Step("factor_scores", "Calculating factor scores")
	done()

	frames := parseSSE(t, rec.Body.String())
	require.Len(t, frames, 2)
	require.Equal(t, "step_started", frames[0].Type)
	require.Equal(t, "factor_scores", frames[0].Step)
	require.Equal(t, "Calculating factor scores", frames[0].Label)
	require.Equal(t, "step_completed", frames[1].Type)
	require.Equal(t, "factor_scores", frames[1].Step)
	// DurationMs is non-negative; we don't assert a tighter bound to keep
	// the test free of timing flakes.
	require.GreaterOrEqual(t, frames[1].DurationMs, int64(0))
}

func TestSSEWriter_RejectsNonFlusher(t *testing.T) {
	_, err := NewSSEWriter(nonFlushingWriter{})
	require.Error(t, err)
}

type fakeReporter struct{ calls []string }

func (f *fakeReporter) Step(id, label string) func() {
	f.calls = append(f.calls, "start:"+id+":"+label)
	return func() { f.calls = append(f.calls, "done:"+id) }
}

// nonFlushingWriter implements http.ResponseWriter but not http.Flusher,
// so we can verify NewSSEWriter rejects it.
type nonFlushingWriter struct{}

func (nonFlushingWriter) Header() http.Header        { return http.Header{} }
func (nonFlushingWriter) Write([]byte) (int, error)  { return 0, nil }
func (nonFlushingWriter) WriteHeader(statusCode int) {}

// parseSSE extracts JSON payloads from SSE `data:` lines so tests can assert
// against the structured Event without re-implementing the SSE format.
func parseSSE(t *testing.T, raw string) []Event {
	t.Helper()
	var out []Event
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var ev Event
		require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev))
		out = append(out, ev)
	}
	return out
}
