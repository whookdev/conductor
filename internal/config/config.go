package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port int
	Host string

	PostgresURL string
	RedisURL    string

	BaseDomain string

	IsDevelopment bool
}

func NewConfig() (*Config, error) {
	godotenv.Load()

	port, err := strconv.Atoi(getEnvWithDefault("PORT", "3000"))
	if err != nil {
		return nil, fmt.Errorf("invalid port: %w", err)
	}

	return &Config{
		Port:          port,
		Host:          getEnvWithDefault("HOST", "0.0.0.0"),
		PostgresURL:   requireEnv("POSTGRES_URL"),
		RedisURL:      requireEnv("REDIS_URL"),
		BaseDomain:    getEnvWithDefault("BASE_DOMAIN", "whook.dev"),
		IsDevelopment: getEnvWithDefault("ENVIRONMENT", "development") == "development",
	}, nil
}

func requireEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		panic(fmt.Sprintf("required environment variable %s is not set", key))
	}

	return val
}

func getEnvWithDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}

	return defaultValue
}
