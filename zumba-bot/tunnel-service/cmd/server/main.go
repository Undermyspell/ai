// tunnel-service: schaltet die öffentlichen ngrok-Tunnel für Wrapped und
// Admin-UI an und aus. Läuft als Sidecar neben dem ngrok-Agenten; das
// Admin-UI spricht nur diesen Service an, nie den Agenten direkt.
package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/michael/zumba-tunnel/internal/config"
	"github.com/michael/zumba-tunnel/internal/controller"
	"github.com/michael/zumba-tunnel/internal/ngrok"
	"github.com/michael/zumba-tunnel/internal/web"
)

// reconcileInterval: wie oft Zustand und Agent abgeglichen werden. Der Aufruf
// geht über localhost, 5s sind billig und halten die Anzeige im Admin-UI
// aktuell.
const reconcileInterval = 5 * time.Second

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Konfiguration: %v", err)
	}

	var agent controller.Agent = ngrok.New(cfg.AgentAPI)
	if cfg.Mock {
		log.Printf("MOCK=true — kein echter ngrok-Agent, Tunnel werden nur simuliert")
		agent = newMockAgent()
	}

	ctrl := controller.New(agent, cfg)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go ctrl.Run(ctx, reconcileInterval)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           web.New(ctrl, cfg).Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		// Beim Herunterfahren alles zumachen: ein Pod, der verschwindet, darf
		// keine offenen Tunnel hinterlassen.
		log.Printf("Beende — schließe offene Tunnel")
		ctrl.CloseAll()
		shutdown, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		waitForClosed(ctrl, shutdown)
		_ = srv.Shutdown(shutdown)
	}()

	for _, t := range cfg.Targets {
		log.Printf("Ziel %q → %s", t.Name, t.Addr)
	}
	log.Printf("tunnel-service auf :%s (Agent %s, TTL %s, max %s)", cfg.Port, cfg.AgentAPI, cfg.DefaultTTL, cfg.MaxTTL)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server: %v", err)
	}
}

// waitForClosed wartet, bis die asynchronen Stop-Aufrufe durch sind — sonst
// endet der Prozess, bevor der Agent die Tunnel abgebaut hat.
func waitForClosed(ctrl *controller.Controller, ctx context.Context) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		busy := false
		for _, s := range ctrl.Status() {
			if s.State != controller.StateInactive && s.State != controller.StateError {
				busy = true
			}
		}
		if !busy {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
