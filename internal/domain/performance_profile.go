package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

func NewPeformanceProfile() *PerformanceProfile {
	return &PerformanceProfile{
		StartTime: time.Now(),
	}
}

type PerformanceProfileEvent struct {
	Name      string                    `json:"name"`
	ElapsedMs int64                     `json:"elapsedMs"`
	Time      time.Time                 `json:"time"`
	Events    []PerformanceProfileEvent `json:"events"`
}

type PerformanceProfile struct {
	StartTime time.Time                 `json:"-"`
	Events    []PerformanceProfileEvent `json:"events"`
	TotalMs   int64                     `json:"totalMs"`
}

func GetPerformanceProfile(ctx context.Context) *PerformanceProfile {
	return ctx.Value(ContextProfileKey).(*PerformanceProfile)
}

func (p *PerformanceProfile) End() {
	p.TotalMs = time.Since(p.StartTime).Milliseconds()
}

func (p *PerformanceProfile) Add(name string) {
	if len(p.Events) == 0 {
		p.Events = append(p.Events, PerformanceProfileEvent{
			Name:      name,
			ElapsedMs: 0,
			Time:      time.Now(),
		})
	}
	lastEvent := p.Events[len(p.Events)-1]
	now := time.Now()
	p.Events = append(p.Events, PerformanceProfileEvent{
		Name:      name,
		ElapsedMs: time.Since(lastEvent.Time).Milliseconds(),
		Time:      now,
	})
}

func pprint(i interface{}) {
	bytes, err := json.MarshalIndent(i, "", "    ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(bytes))
}

func (p PerformanceProfile) Print() {
	p.End()
	pprint(p)
}

func (p PerformanceProfile) ToJsonBytes() ([]byte, error) {
	// i dont think this should ever err
	bytes, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal performance profile: %w", err)
	}
	return bytes, nil
}
