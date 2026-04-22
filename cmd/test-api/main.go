package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"factorbacktest/cmd"
	"factorbacktest/internal/logger"
	"factorbacktest/internal/util"
)

var seedName = flag.String("seed", "", "name of seed to apply at startup")

func main() {
	flag.Parse()

	portStr := os.Getenv("PORT")
	if portStr == "" {
		log.Fatal("PORT env var is required")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		log.Fatalf("invalid PORT %q: %v", portStr, err)
	}
	log.Printf("Test API server will listen on port %d", port)

	testDbManager, err := createTestDbManager(5440)
	if err != nil {
		log.Fatalf("failed to create test database manager: %v", err)
	}
	defer testDbManager.Close()

	if *seedName != "" {
		fn, ok := seeds[*seedName]
		if !ok {
			log.Fatalf("unknown seed %q; known: %v", *seedName, sortedSeedNames())
		}
		fn(testDbManager.DB())
	}

	secrets := util.Secrets{
		Port:             port,
		Db:               testDbManager.DBConfig,
		DataJockeyApiKey: "",
		ChatGPTApiKey:    "",
		Alpaca: util.AlpacaSecrets{
			ApiKey:    "",
			ApiSecret: "",
			Endpoint:  "",
		},
		Jwt: "",
		SES: util.SESSecrets{
			Region:    "",
			FromEmail: "",
		},
	}

	apiHandler, err := cmd.InitializeDependencies(secrets, nil)
	if err != nil {
		log.Fatalf("failed to initialize dependencies: %v", err)
	}
	apiHandler.Port = port

	lg := logger.New()
	ctx := context.WithValue(context.Background(), logger.ContextKey, lg)

	if err := apiHandler.StartApi(ctx); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func createTestDbManager(port int) (*TestDbManager, error) {
	// Generate a unique database name
	timestamp := time.Now().Format("20060102")
	suffix := generateRandomSuffix(4)
	dbName := fmt.Sprintf("%s-%s", timestamp, suffix)

	// Connect to admin database to create test database
	adminConnStr := fmt.Sprintf("postgresql://postgres:postgres@localhost:%d/postgres?sslmode=disable", port)
	adminDb, err := sql.Open("postgres", adminConnStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres admin: %w", err)
	}
	defer adminDb.Close()

	if err := adminDb.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping postgres admin: %w", err)
	}

	// Create the test database
	if err := adminDb.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping postgres admin: %w", err)
	}
	_, err = adminDb.Exec(fmt.Sprintf(`CREATE DATABASE "%s"`, dbName))
	if err != nil {
		return nil, fmt.Errorf("failed to create database %s: %w", dbName, err)
	}

	// Cleanup function to drop database on exit
	dropDb := func() {
		_, err := adminDb.Exec(fmt.Sprintf(`DROP DATABASE "%s" WITH (FORCE)`, dbName))
		if err != nil {
			fmt.Printf("failed to drop test db: %v\n", err)
		}
	}

	// Connect to the test database
	testConfig := util.DbSecrets{
		Host:      "localhost",
		User:      "postgres",
		Port:      fmt.Sprintf("%d", port),
		Password:  "postgres",
		Database:  dbName,
		EnableSsl: false,
	}

	testConnStr := testConfig.ToConnectionStr()
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

	// Run migrations
	if err := runMigrations(testDb); err != nil {
		testDb.Close()
		dropDb()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &TestDbManager{
		dbName:   dbName,
		db:       testDb,
		DBConfig: testConfig,
		dropFn:   dropDb,
		adminDb:  adminDb, // Keep admin DB open for cleanup
	}, nil
}

type TestDbManager struct {
	dbName   string
	db       *sql.DB
	DBConfig util.DbSecrets
	dropFn   func()
	adminDb  *sql.DB
}

func (m *TestDbManager) DB() *sql.DB {
	return m.db
}

func (m *TestDbManager) DbName() string {
	return m.dbName
}

func (m *TestDbManager) Close() error {
	if m.db != nil {
		if err := m.db.Close(); err != nil {
			return err
		}
	}

	if m.adminDb != nil {
		if err := m.adminDb.Close(); err != nil {
			return err
		}
	}

	if m.dropFn != nil {
		m.dropFn()
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
	migrationDir := "./migrations"
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
