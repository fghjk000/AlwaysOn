package config

import (
	"fmt"
	"os"
)

type Config struct {
	DBHost          string
	DBPort          string
	DBUser          string
	DBPassword      string
	DBName          string
	ServerPort      string
	SlackWebhookURL string
	AgentAPIKey     string
}

func Load() *Config {
	return &Config{
		DBHost:          getEnv("DB_HOST", "localhost"),
		DBPort:          getEnv("DB_PORT", "5432"),
		DBUser:          getEnv("DB_USER", "alwayson"),
		DBPassword:      getEnv("DB_PASSWORD", "alwayson123"),
		DBName:          getEnv("DB_NAME", "alwayson"),
		ServerPort:      getEnv("SERVER_PORT", "8080"),
		SlackWebhookURL: getEnv("SLACK_WEBHOOK_URL", ""),
		AgentAPIKey:     getEnv("AGENT_API_KEY", ""),
	}
}

func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName,
	)
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
