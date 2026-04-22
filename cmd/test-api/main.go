package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"factorbacktest/cmd"
	"factorbacktest/internal/logger"
	"factorbacktest/internal/util"
)

func main() {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	actualPort := listener.Addr().(*net.TCPAddr).Port
	log.Printf("Test API server listening on port %d", actualPort)

	// Create a unique test database
	testDbManager, err := createTestDbManager(5440)
	if err != nil {
		log.Fatalf("failed to create test database manager: %v", err)
	}
	defer testDbManager.Close()

	// Initialize API dependencies with test database
	secrets := util.Secrets{
		Port: actualPort,
		Db:   testDbManager.DBConfig,
		// Set other required secrets to empty/zero values for test
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

	lg := logger.New()
	ctx := context.WithValue(context.Background(), logger.ContextKey, lg)

	// Override the port in the API handler
	apiHandler.Port = actualPort

	// Start the server in a goroutine
	go func() {
		err := apiHandler.StartApi(ctx)
		if err != nil {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Output the actual port for the caller to use
	fmt.Printf("%d\n", actualPort)

	// Wait forever (or until interrupted)
	select {}
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
