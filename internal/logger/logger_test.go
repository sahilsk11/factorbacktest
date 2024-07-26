package logger

import (
	"testing"
)

// local util for observing zap behavior
func TestLogger(t *testing.T) {
	// skip in ci checks
	if true {
		t.Skip()
	}

	t.Fail()
}
