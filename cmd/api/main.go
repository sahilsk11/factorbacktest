package main

import (
	"factorbacktest/cmd"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

func main() {
	fmt.Println(os.Getenv("commit_hash"))
	apiHandler, err := cmd.InitializeDependencies()
	if err != nil {
		log.Fatal(err)
	}
	err = apiHandler.StartApi(3009)
	if err != nil {
		log.Fatal(err)
	}
}
