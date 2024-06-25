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
	DataJockeyApiKey string `json:"dataJockey"`
}

func LoadSecrets() (*Secrets, error) {
	f, err := os.ReadFile("secrets.json")
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
