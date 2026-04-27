package main

import (
	"log"
	"os"

	"ap-psych-final/backend/backend"
)

func main() {
	app, err := backend.NewApp("data/app.db", "data/sources")
	if err != nil {
		log.Fatal(err)
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	host := os.Getenv("HOST")
	if host == "" {
		host = "localhost"
	}
	log.Printf("Server is running at http://%s:%s", host, port)
	if err := app.Router().Run(host + ":" + port); err != nil {
		log.Fatal(err)
	}
}
