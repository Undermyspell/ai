package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	Port string

	DB DBConfig

	EvalPeriodStart time.Time
	EvalPeriodEnd   time.Time
}

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

func (d DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode,
	)
}

func Load() (Config, error) {
	cfg := Config{
		Port: getenv("PORT", "8080"),
		DB: DBConfig{
			Host:     getenv("DB_HOST", "192.168.178.46"),
			Port:     getenv("DB_PORT", "5433"),
			User:     getenv("DB_USER", "n8n"),
			Password: getenv("DB_PASSWORD", "n8n_password"),
			Name:     getenv("DB_NAME", "zumba"),
			SSLMode:  getenv("DB_SSLMODE", "disable"),
		},
	}

	start, err := parseDate(getenv("EVAL_PERIOD_START", "2025-12-01"))
	if err != nil {
		return cfg, fmt.Errorf("EVAL_PERIOD_START: %w", err)
	}
	end, err := parseDate(getenv("EVAL_PERIOD_END", "2026-11-30"))
	if err != nil {
		return cfg, fmt.Errorf("EVAL_PERIOD_END: %w", err)
	}
	cfg.EvalPeriodStart = start
	cfg.EvalPeriodEnd = end

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}
