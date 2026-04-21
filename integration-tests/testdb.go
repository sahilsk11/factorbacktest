package integration_tests

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"time"

	_ "github.com/lib/pq"
)

var testDb *sql.DB

type TestDbManager struct {
	dbName string
	db    *sql.DB
}

func NewTestDbManager() (*TestDbManager, error) {
	timestamp := time.Now().Format("20060102")
	suffix := generateRandomSuffix(4)
	dbName := fmt.Sprintf("%s-%s", timestamp, suffix)

	adminConnStr := "postgresql://postgres:postgres@localhost:5440/postgres?sslmode=disable"
	adminDb, err := sql.Open("postgres", adminConnStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres admin: %w", err)
	}
	defer adminDb.Close()

	if err := adminDb.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping postgres admin: %w", err)
	}

	_, err = adminDb.Exec(fmt.Sprintf(`CREATE DATABASE "%s"`, dbName))
	if err != nil {
		return nil, fmt.Errorf("failed to create database %s: %w", dbName, err)
	}

	testConnStr := fmt.Sprintf("postgresql://postgres:postgres@localhost:5440/%s?sslmode=disable", dbName)
	testDb, err := sql.Open("postgres", testConnStr)
	if err != nil {
		adminDb.Exec(fmt.Sprintf(`DROP DATABASE "%s" WITH (FORCE)`, dbName))
		return nil, fmt.Errorf("failed to connect to test database: %w", err)
	}

	if err := testDb.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping test database: %w", err)
	}

	migrationCmd := exec.Command("python3", "tools/migrations.py", "up", dbName)
	migrationCmd.Dir = getProjectRoot()
	migrationCmd.Env = append(os.Environ(), "ALPHA_ENV=test")
	output, err := migrationCmd.CombinedOutput()
	if err != nil {
		testDb.Close()
		adminDb.Exec(fmt.Sprintf(`DROP DATABASE "%s" WITH (FORCE)`, dbName))
		return nil, fmt.Errorf("failed to run migrations: %w\noutput: %s", err, string(output))
	}

	return &TestDbManager{
		dbName: dbName,
		db:    testDb,
	}, nil
}

func (m *TestDbManager) DB() *sql.DB {
	return m.db
}

func (m *TestDbManager) DbName() string {
	return m.dbName
}

func (m *TestDbManager) Close() error {
	if m.db != nil {
		m.db.Close()
	}

	adminConnStr := "postgresql://postgres:postgres@localhost:5440/postgres?sslmode=disable"
	adminDb, err := sql.Open("postgres", adminConnStr)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres admin: %w", err)
	}
	defer adminDb.Close()

	_, err = adminDb.Exec(fmt.Sprintf(`DROP DATABASE "%s" WITH (FORCE)`, m.dbName))
	if err != nil {
		return fmt.Errorf("failed to drop database %s: %w", m.dbName, err)
	}

	return nil
}

func SetTestDb(db *sql.DB) {
	testDb = db
}

func GetTestDb() *sql.DB {
	return testDb
}

func generateRandomSuffix(length int) string {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	seed := int(time.Now().UnixNano())
	result := make([]byte, length)
	charsetLen := len(charset)
	for i := 0; i < length; i++ {
		seed = seed*1103515245 + 12345
		result[i] = charset[(seed/65536)%charsetLen]
	}
	return string(result)
}

func getProjectRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}