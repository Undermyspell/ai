// Package config liest die Umgebung des tunnel-service.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Target ist ein freischaltbares Ziel. Die Adresse steht hier und kommt
// niemals aus dem Request — der Aufrufer wählt nur den Namen. Sonst könnte,
// wer den Service erreicht, jeden Dienst im Namespace ins Netz stellen
// (Postgres inklusive).
type Target struct {
	Name  string `json:"name"`
	Label string `json:"label"`
	Addr  string `json:"addr"`
}

type Config struct {
	Port string

	// AgentAPI ist die lokale ngrok-Agent-API. Der Agent läuft als zweiter
	// Container im selben Pod und lauscht auf 127.0.0.1 — erreichbar also nur
	// von hier aus, nicht aus dem restlichen Cluster.
	AgentAPI string

	Targets []Target

	// DefaultTTL gilt, wenn der Aufrufer keine Laufzeit mitschickt, MaxTTL ist
	// die Obergrenze. Ein Tunnel ohne Ablauf wäre genau das, was wir nicht
	// wollen: „temporär" darf nicht am Erinnern hängen.
	DefaultTTL time.Duration
	MaxTTL     time.Duration

	// Mock ersetzt den ngrok-Agent durch eine Attrappe (lokale Entwicklung,
	// kein Authtoken nötig).
	Mock bool
}

func Load() (Config, error) {
	cfg := Config{
		Port:     getenv("PORT", "8080"),
		AgentAPI: getenv("AGENT_API", "http://127.0.0.1:4040"),
		Mock:     os.Getenv("MOCK") == "true",
	}

	raw := getenv("TUNNEL_TARGETS", "[]")
	if err := json.Unmarshal([]byte(raw), &cfg.Targets); err != nil {
		return Config{}, fmt.Errorf("TUNNEL_TARGETS ist kein gültiges JSON: %w", err)
	}
	if len(cfg.Targets) == 0 {
		return Config{}, fmt.Errorf("TUNNEL_TARGETS ist leer — ohne Ziele hat der Service nichts zu tun")
	}
	seen := map[string]bool{}
	for _, t := range cfg.Targets {
		if t.Name == "" || t.Addr == "" {
			return Config{}, fmt.Errorf("Ziel ohne name oder addr: %+v", t)
		}
		if seen[t.Name] {
			return Config{}, fmt.Errorf("Ziel %q doppelt", t.Name)
		}
		seen[t.Name] = true
	}

	var err error
	if cfg.DefaultTTL, err = duration("DEFAULT_TTL", 2*time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.MaxTTL, err = duration("MAX_TTL", 24*time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.DefaultTTL > cfg.MaxTTL {
		return Config{}, fmt.Errorf("DEFAULT_TTL (%s) ist größer als MAX_TTL (%s)", cfg.DefaultTTL, cfg.MaxTTL)
	}
	return cfg, nil
}

func duration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s muss positiv sein", key)
	}
	return d, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
