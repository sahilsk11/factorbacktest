package internal

import (
	"encoding/json"
	"fmt"
	"os"
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

func LoadSecrets() (*Secrets, error) {
	secretsFile := "secrets.json"
	if os.Getenv("ALPHA_ENV") == "dev" {
		// secretsFile = "secrets-dev.json"
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
