package calculator

import (
	"math"
	"testing"
)

func Test_percentChange(t *testing.T) {
	tests := []struct {
		name     string
		end      float64
		start    float64
		expected float64
	}{
		{
			name:     "simple increase",
			end:      110,
			start:    100,
			expected: 10.0,
		},
		{
			name:     "simple decrease",
			end:      90,
			start:    100,
			expected: -10.0,
		},
		{
			name:     "no change",
			end:      100,
			start:    100,
			expected: 0.0,
		},
		{
			name:     "double",
			end:      200,
			start:    100,
			expected: 100.0,
		},
		{
			name:     "50 percent drop",
			end:      50,
			start:    100,
			expected: -50.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := percentChange(tt.end, tt.start)
			if math.Abs(got-tt.expected) > 0.0001 {
				t.Errorf("percentChange(%v, %v) = %v, want %v", tt.end, tt.start, got, tt.expected)
			}
		})
	}
}
