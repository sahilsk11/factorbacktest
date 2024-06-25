package main

import (
	"factorbacktest/cmd"
	"log"

	_ "github.com/lib/pq"
)

func main() {
	apiHandler, err := cmd.InitializeDependencies()
	if err != nil {
		log.Fatal(err)
	}
	err = apiHandler.StartApi(3009)
	if err != nil {
		log.Fatal(err)
	}
}
