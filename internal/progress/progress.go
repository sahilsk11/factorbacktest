// Package progress lets long-running handlers emit user-facing step events
// (e.g. "calculating factor scores") without coupling the business logic to a
// specific transport. The default Reporter is a no-op so the same code path
// works for the existing synchronous /backtest endpoint and the new
// /backtest/stream SSE endpoint.
package progress

import (
	"context"
	"time"
)

// Event is the wire-level shape we serialize to clients (currently as JSON
// inside SSE `data:` frames). It intentionally covers progress events,
// terminal results, and terminal errors so a single stream conveys everything
// a client needs.
type Event struct {
	// Type is one of: "step_started", "step_completed", "result", "error".
	Type string `json:"type"`

	// Step is a stable identifier (e.g. "factor_scores"). Set on
	// step_started / step_completed.
	Step string `json:"step,omitempty"`

	// Label is the human-friendly description for the step. Sent on
	// step_started so the UI can render it without needing a client-side
	// id->label table.
	Label string `json:"label,omitempty"`

	// DurationMs is the elapsed time for the step. Set on step_completed.
	DurationMs int64 `json:"durationMs,omitempty"`

	// Result carries the final response payload on a "result" event.
	Result any `json:"result,omitempty"`

	// Error is the human-readable error string on an "error" event.
	Error string `json:"error,omitempty"`
}

// Reporter receives progress events. Implementations must be safe to call
// from a single goroutine; we do not attempt to serialize concurrent calls.
type Reporter interface {
	// Step marks a new logical step as started. The returned function
	// MUST be called when the step finishes (success or failure); it
	// emits the matching step_completed event with the elapsed time.
	// Calling the returned function more than once is a no-op.
	Step(id, label string) (done func())
}

// noop is the default Reporter used when the caller hasn't installed one.
// It exists so business code can unconditionally call progress.Step(ctx, ...)
// without nil checks.
type noop struct{}

func (noop) Step(string, string) func() { return func() {} }

// Noop returns a Reporter that drops every event. Useful for tests and for
// the legacy synchronous /backtest endpoint.
func Noop() Reporter { return noop{} }

type ctxKey struct{}

// WithReporter returns a new context carrying r. Pass this context through
// the call chain so deeply-nested code can emit progress without taking a
// Reporter parameter on every function signature.
func WithReporter(ctx context.Context, r Reporter) context.Context {
	if r == nil {
		r = Noop()
	}
	return context.WithValue(ctx, ctxKey{}, r)
}

// FromContext extracts the Reporter installed by WithReporter, or returns
// Noop() when none is present. This is the function business code should
// call.
func FromContext(ctx context.Context) Reporter {
	if r, ok := ctx.Value(ctxKey{}).(Reporter); ok && r != nil {
		return r
	}
	return Noop()
}

// Step is a convenience wrapper so callers can write
//
//	done := progress.Step(ctx, "factor_scores", "Calculating factor scores")
//	defer done()
//
// without first pulling the Reporter out of context.
func Step(ctx context.Context, id, label string) func() {
	return FromContext(ctx).Step(id, label)
}

// nowMs is overridable in tests; we use it instead of time.Since so
// implementations can share clock injection if we want it later.
var nowMs = func() int64 { return time.Now().UnixMilli() }
