package logger

import (
	"fmt"
	"testing"
)

func TestLogger(t *testing.T) {
	// skip in ci checks
	if true {
		t.Skip()
	}

	Info("hello")

	Error(fmt.Errorf("ah man"))

	t.Fail()
}
