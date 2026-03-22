package config

import "os"

type TelegramConfig struct {
	BotToken      string
	DefaultChatID string
}

type Config struct {
	Port         string
	DatabasePath string
	Enviroment   string
	Telegram     TelegramConfig
}

func LoadConfig() *Config {
	port := getEnv("PORT", "8080")
	dbPath := getEnv("DATABASE_DSN", "host=localhost user=postgres password=postgres dbname=capyflow port=5432 sslmode=disable")
	env := getEnv("ENVIROMENT", "development")

	telegram := TelegramConfig{
		BotToken:      getEnv("TELEGRAM_BOT_TOKEN", ""),
		DefaultChatID: getEnv("TELEGRAM_DEFAULT_CHAT_ID", ""),
	}

	return &Config{
		Port:         port,
		DatabasePath: dbPath,
		Enviroment:   env,
		Telegram:     telegram,
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	return value
}
