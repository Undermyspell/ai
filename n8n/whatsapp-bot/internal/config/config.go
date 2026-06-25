package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	Port string

	DB DBConfig

	Gemini    GeminiConfig
	Evolution EvolutionConfig

	// Output steuert, wohin erzeugte WhatsApp-Nachrichten gehen.
	Output OutputConfig

	// GroupJID ist die remoteJid der Zumba-WhatsApp-Gruppe. Nur Nachrichten aus
	// dieser Gruppe werden klassifiziert (1:1 aus dem n8n-Workflow).
	GroupJID string

	// PreviewJID ist die Nummer, an die der "Vorschau"-Modus der Bot-Test-Seite
	// die erzeugte Nachricht schickt (statt an die Gruppe). Leer = Vorschau aus.
	PreviewJID string

	// Location steuert die Donnerstag-Prüfung und das Tagesdatum für die DB-Writes.
	Location *time.Location
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

type GeminiConfig struct {
	APIKey        string
	Model         string // Primärmodell (n8n: "Gemine 2.5-flash", Index 0)
	FallbackModel string // Fallback (n8n: "Gemini-3-flash-preview", Index 1)
}

type EvolutionConfig struct {
	URL      string
	APIKey   string
	Instance string
}

// OutputMode bestimmt das Ziel ausgehender WhatsApp-Nachrichten.
type OutputMode string

const (
	OutputEvolution OutputMode = "evolution" // an die Evolution API senden (Default)
	OutputStdout    OutputMode = "stdout"    // nach stdout schreiben (lokaler Test)
	OutputFile      OutputMode = "file"      // an eine Datei anhängen (lokaler Test)
)

type OutputConfig struct {
	Mode OutputMode
	File string // Pfad bei Mode == file
}

func Load() (Config, error) {
	tz := getenv("TZ", "Europe/Berlin")
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return Config{}, fmt.Errorf("TZ %q: %w", tz, err)
	}

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
		Gemini: GeminiConfig{
			APIKey:        os.Getenv("GEMINI_API_KEY"),
			Model:         getenv("GEMINI_MODEL", "gemini-2.5-flash"),
			FallbackModel: getenv("GEMINI_FALLBACK_MODEL", "gemini-3-flash-preview"),
		},
		Evolution: EvolutionConfig{
			URL:      getenv("EVOLUTION_URL", "http://localhost:8090"),
			APIKey:   getenv("EVOLUTION_API_KEY", "test_api_key"),
			Instance: getenv("EVOLUTION_INSTANCE", "whatsapp"),
		},
		Output: OutputConfig{
			Mode: OutputMode(getenv("OUTPUT_MODE", string(OutputEvolution))),
			File: getenv("OUTPUT_FILE", "output.txt"),
		},
		GroupJID:   getenv("ZUMBA_GROUP_JID", "000000000000-0000000000@g.us"),
		PreviewJID: os.Getenv("PREVIEW_JID"),
		Location:   loc,
	}

	switch cfg.Output.Mode {
	case OutputEvolution, OutputStdout, OutputFile:
	default:
		return Config{}, fmt.Errorf("OUTPUT_MODE %q: erlaubt sind evolution|stdout|file", cfg.Output.Mode)
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
