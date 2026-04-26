package progress

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// SSEWriter encodes Events as Server-Sent Event frames and flushes after
// every write so clients see steps in real time. It is NOT safe for
// concurrent use; we expect a single goroutine (the request handler) to
// drive the backtest sequentially.
type SSEWriter struct {
	w   io.Writer
	flu http.Flusher

	mu     sync.Mutex
	closed bool
}

// NewSSEWriter wraps a ResponseWriter that supports flushing. The caller is
// responsible for setting SSE response headers (Content-Type, Cache-Control,
// etc.) before invoking this — we don't touch headers here so this struct
// stays decoupled from gin.
func NewSSEWriter(w http.ResponseWriter) (*SSEWriter, error) {
	flu, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("response writer does not support flushing; SSE requires http.Flusher")
	}
	return &SSEWriter{w: w, flu: flu}, nil
}

// Send marshals e to JSON and writes a single SSE `data:` frame. We don't
// use the optional `event:` line — a single typed JSON payload is simpler to
// parse on the FE than juggling multiple SSE event names.
func (s *SSEWriter) Send(e Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("sse writer closed")
	}
	body, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal sse event: %w", err)
	}
	if _, err := fmt.Fprintf(s.w, "data: %s\n\n", body); err != nil {
		return err
	}
	s.flu.Flush()
	return nil
}

// Close marks the writer as no longer accepting events. Subsequent Send
// calls return an error. We don't actually close the underlying connection
// here; gin owns that.
func (s *SSEWriter) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
}

// SSEReporter is a Reporter that emits step_started / step_completed events
// to the supplied SSEWriter. Send errors are swallowed: if the client
// disconnects mid-stream we still want the in-flight backtest to finish
// (cheaply) without crashing the goroutine.
type SSEReporter struct {
	w *SSEWriter
}

// NewSSEReporter binds a Reporter to an SSEWriter.
func NewSSEReporter(w *SSEWriter) *SSEReporter { return &SSEReporter{w: w} }

func (r *SSEReporter) Step(id, label string) func() {
	start := nowMs()
	_ = r.w.Send(Event{
		Type:  "step_started",
		Step:  id,
		Label: label,
	})
	var once sync.Once
	return func() {
		once.Do(func() {
			_ = r.w.Send(Event{
				Type:       "step_completed",
				Step:       id,
				DurationMs: nowMs() - start,
			})
		})
	}
}
