package config

import "os"

type Config struct {
	Port         string
	DatabasePath string
	Enviroment   string
}

func LoadConfig() *Config {
	port := getEnv("PORT", "8080")
	dbPath := getEnv("DATABASE_DSN", "host=localhost user=postgres password=postgres dbname=capyflow port=5432 sslmode=disable")
	env := getEnv("ENVIROMENT", "development")

	return &Config{
		Port:         port,
		DatabasePath: dbPath,
		Enviroment:   env,
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	return value
}
