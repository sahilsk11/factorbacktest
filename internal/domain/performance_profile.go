package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type PerformanceProfileEvent struct {
	Name      string `json:"name"`
	ElapsedMs int64  `json:"elapsed"`
	Time      time.Time
}

type PerformanceProfile struct {
	Events []PerformanceProfileEvent `json:"events"`
	Total  int64                     `json:"total"`
}

func GetPerformanceProfile(ctx context.Context) *PerformanceProfile {
	return ctx.Value("performanceProfile").(*PerformanceProfile)
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
	p.Total = p.Events[len(p.Events)-1].Time.Sub(p.Events[0].Time).Milliseconds()
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
