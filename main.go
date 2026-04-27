package main

import (
	"log"

	"ap-psych-final/backend/backend"
)

func main() {
	app, err := backend.NewApp("data/app.db", "data/sources")
	if err != nil {
		log.Fatal(err)
	}
	if err := app.Router().Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
