// Package config reads process settings from the environment.
package config

import "os"

// LocalDatabaseURL is the compose Postgres published on the host.
const LocalDatabaseURL = "postgres://challenge:challenge@127.0.0.1:5432/challenge?sslmode=disable"

// Config is the process configuration.
type Config struct {
	DatabaseURL  string
	HTTPAddr     string
	SeedAccounts bool
}

// Load reads configuration. An empty DATABASE_URL uses the host port of compose Postgres.
// SeedAccounts is on only when SEED_ACCOUNTS=true.
func Load() Config {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = LocalDatabaseURL
	}
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":3000"
	}
	return Config{
		DatabaseURL:  url,
		HTTPAddr:     addr,
		SeedAccounts: os.Getenv("SEED_ACCOUNTS") == "true",
	}
}
