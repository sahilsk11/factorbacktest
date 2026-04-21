package integration_tests

import (
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"factorbacktest/internal/util"

	_ "github.com/lib/pq"
)

type TestDbManager struct {
	dbName   string
	db       *sql.DB
	DBConfig util.DbSecrets
}

func NewTestDbManager() (*TestDbManager, error) {
	timestamp := time.Now().Format("20060102")
	suffix := generateRandomSuffix(4)
	dbName := fmt.Sprintf("%s-%s", timestamp, suffix)

	dbConfig := util.DbSecrets{
		Host:      "localhost",
		User:      "postgres",
		Port:      "5440",
		Password:  "postgres",
		Database:  dbName,
		EnableSsl: false,
	}

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

	dropDb := func() {
		_, err := adminDb.Exec(fmt.Sprintf(`DROP DATABASE "%s" WITH (FORCE)`, dbName))
		if err != nil {
			fmt.Println("failed to drop test db:", err)
		}
	}

	testConnStr := dbConfig.ToConnectionStr()
	testDb, err := sql.Open("postgres", testConnStr)
	if err != nil {
		dropDb()
		return nil, fmt.Errorf("failed to connect to test database: %w", err)
	}

	if err := testDb.Ping(); err != nil {
		_ = testDb.Close()
		dropDb()
		return nil, fmt.Errorf("failed to ping test database: %w", err)
	}

	if err := runMigrations(testDb); err != nil {
		testDb.Close()
		dropDb()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &TestDbManager{
		dbName:   dbName,
		db:       testDb,
		DBConfig: dbConfig,
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

func generateRandomSuffix(length int) string {
	charset := "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}

func runMigrations(db *sql.DB) error {
	migrationDir := filepath.Join(getProjectRoot(), "migrations")
	entries, err := os.ReadDir(migrationDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".sql" && strings.HasSuffix(e.Name(), ".up.sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, file := range files {
		content, err := os.ReadFile(filepath.Join(migrationDir, file))
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", file, err)
		}
		if _, err := db.Exec(string(content)); err != nil {
			return fmt.Errorf("failed to execute %s: %w", file, err)
		}
	}
	return nil
}

func getProjectRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return filepath.Dir(wd)
}
