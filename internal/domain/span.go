package domain

import (
	"context"
	"encoding/json"
	"time"
)

type Span struct {
	Name       string    `json:"name"`
	startTs    time.Time `json:"-"`
	subProfile *Profile  `json:"-"`

	SubSpans []*Span `json:"subSpans,omitempty"`
	Elapsed  *int64  `json:"elapsed"`
}

const ContextProfileKey = "performanceProfile"

func GetProfile(ctx context.Context) (profile *Profile, endNewProfile func()) {
	profile = ctx.Value(ContextProfileKey).(*Profile)
	return profile, profile.End
}

// Profile is simply a list of spans
type Profile struct {
	Spans   []*Span
	startTs time.Time
	TotalMs *int64
}

func (p *Profile) End() {
	t := time.Since(p.startTs).Milliseconds()
	if p.TotalMs == nil {
		p.TotalMs = &t
	}
}

func (s *Span) End() {
	if s.Elapsed == nil {
		t := time.Since(s.startTs).Milliseconds()
		s.Elapsed = &t
	}
	if s.subProfile != nil {
		s.SubSpans = s.subProfile.Spans
	}
}

func NewProfile() (newProfile *Profile, endNewProfile func()) {
	newProfile = &Profile{
		Spans:   []*Span{},
		startTs: time.Now(),
	}

	return newProfile, newProfile.End
}

// only exposing to make thread-safe
func NewSpan(name string) (*Span, func()) {
	newSpan := &Span{
		Name:    name,
		startTs: time.Now(),
	}
	return newSpan, newSpan.End
}

// only exposing to make thread-safe
func (p *Profile) AddSpan(s *Span) {
	p.Spans = append(p.Spans, s)
}

// StartNewSpan ends the last span and begins a new one
// not thread safe
func (p *Profile) StartNewSpan(name string) (newSpan *Span, endSpan func()) {
	newSpan, endSpan = NewSpan(name)
	if len(p.Spans) > 0 {
		p.Spans[len(p.Spans)-1].End()
	}
	p.Spans = append(p.Spans, newSpan)
	return newSpan, endSpan
}

// do we need to expose this?
func (s *Span) NewSubProfile() (*Profile, func()) {
	if s.subProfile != nil {
		panic("attempting to override existing subprofile")
	}
	newProfile, end := NewProfile()
	s.subProfile = newProfile
	return newProfile, end
}

func (p *Profile) ToJsonBytes() ([]byte, error) {
	bytes, err := json.Marshal(p.Spans)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

// i feel like this should create a new span too, just a profile
func NewCtxWithSubProfile(ctx context.Context, parentSpan *Span) context.Context {
	newProfile, _ := parentSpan.NewSubProfile()
	return context.WithValue(ctx, ContextProfileKey, newProfile)
}
