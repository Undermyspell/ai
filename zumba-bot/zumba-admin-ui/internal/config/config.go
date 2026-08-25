package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port string

	DB DBConfig

	// BotURL ist die Basis-URL des whatsapp-bot (für die Bot-Test-Seite).
	BotURL string

	// ClassifierURL ist die Basis-URL des classifier-service (für den
	// manuellen ML-Test). Leer = Seite meldet "nicht konfiguriert".
	ClassifierURL string
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
			Password: os.Getenv("DB_PASSWORD"),
			Name:     getenv("DB_NAME", "zumba"),
			SSLMode:  getenv("DB_SSLMODE", "disable"),
		},
		BotURL:        getenv("BOT_URL", "http://localhost:8080"),
		ClassifierURL: os.Getenv("CLASSIFIER_URL"),
	}
	// Der Auswertungszeitraum kommt NICHT mehr aus der Umgebung, sondern aus
	// der Tabelle public.seasons – sonst müsste zu jedem Jahreswechsel
	// deployt werden, und Archiv-Ansichten wären unmöglich.
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
