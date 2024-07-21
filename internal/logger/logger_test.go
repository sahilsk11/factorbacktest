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

	Info("hello %s", "sahil")

	Error(fmt.Errorf("ah man"))

	t.Fail()
}