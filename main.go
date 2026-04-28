package main

import (
	"bufio"
	"log"
	"os"
	"strings"

	"ap-psych-final/backend/backend"
)

func main() {
	loadDotEnv(".env")
	requireDeployConfig()

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

func requireDeployConfig() {
	if !productionMode() {
		return
	}
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret == "" || secret == "local-dev-secret-change-me" || len(secret) < 32 {
		log.Fatal("JWT_SECRET must be set to a unique value of at least 32 characters before production deployment")
	}
}

func productionMode() bool {
	return envEnabled("APP_REQUIRE_SECURE_CONFIG") ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("GIN_MODE")), "release")
}

func envEnabled(key string) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func loadDotEnv(file string) {
	f, err := os.Open(file)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" || os.Getenv(key) != "" {
			continue
		}
		_ = os.Setenv(key, value)
	}
}
