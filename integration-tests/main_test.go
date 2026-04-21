package integration_tests

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	manager, err := NewTestDbManager()
	if err != nil {
		os.Stderr.WriteString("Failed to create test database: " + err.Error() + "\n")
		os.Exit(1)
	}

	SetTestDb(manager.DB())

	code := m.Run()

	if err := manager.Close(); err != nil {
		os.Stderr.WriteString("Failed to close test database: " + err.Error() + "\n")
		os.Exit(1)
	}

	os.Exit(code)
}