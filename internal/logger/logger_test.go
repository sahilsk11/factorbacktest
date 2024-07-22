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

	x := map[string]string{
		"hi": "ok",
	}
	Info("hi %v", x)

	Error(fmt.Errorf("ah man"))

	t.Fail()
}
