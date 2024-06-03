package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

func Pprint(i interface{}) {
	bytes, err := json.MarshalIndent(i, "", "    ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(bytes))
}

type Secrets struct {
	DataJockeyApiKey string    `json:"dataJockey"`
	ChatGPTApiKey    string    `json:"gpt"`
	Db               DbSecrets `json:"db"`
}

type DbSecrets struct {
	Host      string `json:"host"`
	User      string `json:"user"`
	Port      string `json:"port"`
	Password  string `json:"password"`
	Database  string `json:"database"`
	EnableSsl bool   `json:"enableSsl"`
}

func (t DbSecrets) ToConnectionStr() string {
	x := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s",
		t.Host, t.Port, t.User, t.Password, t.Database)
	if !t.EnableSsl {
		x += " sslmode=disable"
	}
	return x
}

func NewTestDb() (*sql.DB, error) {
	connStr := "postgresql://postgres:postgres@localhost:5440/postgres_test?sslmode=disable"
	dbConn, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	return dbConn, nil
}

func LoadSecrets() (*Secrets, error) {
	secretsFile := "/go/src/app/secrets.json"
	if os.Getenv("ALPHA_ENV") == "dev" {
		secretsFile = "secrets-dev.json"
	} else if os.Getenv("ALPHA_ENV") == "test" {
		secretsFile = "secrets-test.json"
	}
	f, err := os.ReadFile(secretsFile)
	if err != nil {
		return nil, fmt.Errorf("could not open secrets.json: %w", err)
	}

	secrets := Secrets{}
	err = json.Unmarshal(f, &secrets)
	if err != nil {
		return nil, err
	}

	return &secrets, nil
}

type PerformanceProfileEvent struct {
	Name      string `json:"name"`
	ElapsedMs int64  `json:"elapsed"`
	Time      time.Time
}

type PerformanceProfile struct {
	Events []PerformanceProfileEvent `json:"events"`
	Total  int64                     `json:"total"`
}

func GetPerformanceProfile(ctx context.Context) *PerformanceProfile {
	return ctx.Value("performanceProfile").(*PerformanceProfile)
}

func (p *PerformanceProfile) Add(name string) {
	if len(p.Events) == 0 {
		p.Events = append(p.Events, PerformanceProfileEvent{
			Name:      name,
			ElapsedMs: 0,
			Time:      time.Now(),
		})
	}
	lastEvent := p.Events[len(p.Events)-1]
	now := time.Now()
	p.Events = append(p.Events, PerformanceProfileEvent{
		Name:      name,
		ElapsedMs: time.Since(lastEvent.Time).Milliseconds(),
		Time:      now,
	})
}

func (p PerformanceProfile) Print() {
	p.Total = p.Events[len(p.Events)-1].Time.Sub(p.Events[0].Time).Milliseconds()
	Pprint(p)
}
