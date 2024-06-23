package domain

import (
	"context"
	"encoding/json"
	"time"
)

type Span struct {
	Name     string    `json:"name"`
	startTs  time.Time `json:"-"`
	SubSpans []*Span   `json:"subSpans"`

	Elapsed *int64 `json:"elapsed"`
}

const ContextProfileKey = "performanceProfile"

func GetProfile(ctx context.Context) *Profile {
	return ctx.Value(ContextProfileKey).(*Profile)
}

type Profile struct {
	Spans   []*Span
	startTs time.Time
	TotalMs *int64
}

func (p *Profile) End() {
	t := time.Since(p.startTs).Milliseconds()
	if p.TotalMs != nil {
		p.TotalMs = &t
	}
}

func (s *Span) End() {
	if s.Elapsed != nil {
		t := time.Since(s.startTs).Milliseconds()
		s.Elapsed = &t
	}
}

func NewProfile() *Profile {
	return &Profile{
		Spans: []*Span{},
	}
}

// StartNewSpan ends the last span and begins a new one
func (p *Profile) StartNewSpan(name string) *Span {
	newSpan := &Span{
		Name:     name,
		startTs:  time.Now(),
		SubSpans: []*Span{},
	}
	if len(p.Spans) > 0 {
		p.Spans[len(p.Spans)-1].End()
	}
	p.Spans = append(p.Spans, newSpan)
	return newSpan
}

func (s *Span) NewSubProfile() *Profile {
	return &Profile{
		Spans: s.SubSpans,
	}
}

func (p *Profile) ToJsonBytes() ([]byte, error) {
	bytes, err := json.Marshal(p.Spans)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

func NewCtxWithSubProfile(ctx context.Context, parentSpan *Span) context.Context {
	return context.WithValue(ctx, ContextProfileKey, parentSpan.NewSubProfile())
}
