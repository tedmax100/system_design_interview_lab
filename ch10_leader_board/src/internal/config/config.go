package config

import (
	"os"
	"strconv"
)

type Config struct {
	UseRedis bool
	DB       DBConfig
	Redis    RedisConfig
}

type DBConfig struct {
	DSN string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

func Load() *Config {
	useRedis, _ := strconv.ParseBool(getEnv("USE_REDIS", "false"))

	return &Config{
		UseRedis: useRedis,
		DB: DBConfig{
			DSN: getEnv("DATABASE_URL", "postgres://postgres:postgres123@postgresql:5432/leaderboard?sslmode=disable"),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "valkey:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       0,
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
