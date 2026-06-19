package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/joho/godotenv"

	"github.com/michael/zumba-whatsapp-bot/internal/classifier"
	"github.com/michael/zumba-whatsapp-bot/internal/config"
	"github.com/michael/zumba-whatsapp-bot/internal/db"
	"github.com/michael/zumba-whatsapp-bot/internal/evolution"
	"github.com/michael/zumba-whatsapp-bot/internal/sink"
	"github.com/michael/zumba-whatsapp-bot/internal/store"
	"github.com/michael/zumba-whatsapp-bot/internal/web"
)

func main() {
	if err := godotenv.Load(); err == nil {
		log.Printf("📄 Loaded .env")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Anders als die Schwester-Services gibt es hier KEIN Mock-Fallback:
	// ein Bot ohne DB ist sinnlos.
	pg, err := db.Open(cfg.DB)
	if err != nil {
		log.Fatalf("❌ DB unreachable: %v", err)
	}
	defer pg.Close()
	log.Printf("✅ Connected to PostgreSQL '%s' on %s:%s", cfg.DB.Name, cfg.DB.Host, cfg.DB.Port)

	st := store.NewPostgres(pg)
	cl := classifier.NewGemini(cfg.Gemini.APIKey, cfg.Gemini.Model, cfg.Gemini.FallbackModel)

	var snd web.Sender
	switch cfg.Output.Mode {
	case config.OutputStdout:
		snd = sink.NewStdout()
		log.Printf("📤 Output-Modus: stdout")
	case config.OutputFile:
		snd = sink.NewFile(cfg.Output.File)
		log.Printf("📤 Output-Modus: file (%s)", cfg.Output.File)
	default:
		snd = evolution.NewClient(cfg.Evolution.URL, cfg.Evolution.APIKey, cfg.Evolution.Instance)
		log.Printf("📤 Output-Modus: evolution (%s)", cfg.Evolution.URL)
	}

	srv := web.New(st, cl, snd, cfg.GroupJID, cfg.Location)

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("🍻 Zumba WhatsApp-Bot läuft auf http://localhost%s (Webhook: POST /webhook/whatsapp)", addr)
	if err := http.ListenAndServe(addr, srv.Routes()); err != nil {
		log.Fatal(err)
	}
}
