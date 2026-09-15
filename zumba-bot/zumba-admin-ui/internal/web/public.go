package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/michael/zumba-admin-ui/internal/tunnel"
	"github.com/michael/zumba-admin-ui/web/templates/public"
)

// tunnelClient ist der Ausschnitt des tunnel-service, den die Seite braucht
// (als Interface, damit Tests ohne laufenden Dienst auskommen).
type tunnelClient interface {
	Status(ctx context.Context) (tunnel.Status, error)
	Open(ctx context.Context, target string, ttl time.Duration) (tunnel.Status, error)
	Close(ctx context.Context, target string) (tunnel.Status, error)
	CloseAll(ctx context.Context) (tunnel.Status, error)
}

// publicGate merkt sich, ob gerade ein Tunnel offen ist. Danach richtet sich
// die Sperre von Bot- und ML-Test: die Bot-Test-Seite kann im Preview-Modus
// echte WhatsApp-Nachrichten auslösen, und das soll niemand können, der
// zufällig auf der öffentlichen Adresse landet.
//
// Der Wert wird kurz zwischengespeichert, sonst ginge bei jedem Seitenaufruf
// eine Anfrage an den tunnel-service. Ist der nicht erreichbar, bleibt der
// letzte bekannte Stand stehen.
type publicGate struct {
	mu      sync.Mutex
	open    bool
	checked time.Time
}

const gateTTL = 5 * time.Second

func (g *publicGate) set(open bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.open = open
	g.checked = time.Now()
}

func (g *publicGate) get() (open bool, fresh bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.open, time.Since(g.checked) < gateTTL
}

// publicOpen sagt, ob gerade etwas öffentlich hängt.
func (s *Server) publicOpen(ctx context.Context) bool {
	if s.tunnel == nil {
		return false
	}
	if open, fresh := s.gate.get(); fresh {
		return open
	}
	status, err := s.tunnel.Status(ctx)
	if err != nil {
		log.Printf("tunnel status: %v", err)
		open, _ := s.gate.get()
		return open // letzter bekannter Stand
	}
	s.gate.set(status.AnyOpen())
	return status.AnyOpen()
}

// blockWhilePublic sperrt Seiten, die nach außen nicht offenstehen dürfen.
func (s *Server) blockWhilePublic(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.publicOpen(r.Context()) {
			s.triggerToast(w, "error", "Gesperrt, solange ein öffentlicher Tunnel offen ist.")
			http.Error(w, "Diese Seite ist gesperrt, solange ein öffentlicher Tunnel offen ist. Unter „Öffentlich“ schließen.", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handlePublic(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, s.meta("Öffentlich", "public"), public.Page(s.publicVM(r.Context())))
}

func (s *Server) handlePublicStatus(w http.ResponseWriter, r *http.Request) {
	s.renderPublicCards(w, r)
}

func (s *Server) handlePublicOpen(w http.ResponseWriter, r *http.Request) {
	target := r.FormValue("target")
	ttl := time.Duration(0)
	if h, err := strconv.Atoi(r.FormValue("hours")); err == nil && h > 0 {
		ttl = time.Duration(h) * time.Hour
	}

	status, err := s.tunnel.Open(r.Context(), target, ttl)
	switch {
	case errors.Is(err, tunnel.ErrBusy):
		s.triggerToast(w, "error", "Gerade in Arbeit — kurz warten.")
	case err != nil:
		log.Printf("tunnel open %s: %v", target, err)
		s.triggerToast(w, "error", "Öffnen fehlgeschlagen: "+err.Error())
	default:
		s.gate.set(status.AnyOpen())
		s.triggerToast(w, "success", "Tunnel wird aufgebaut.")
	}
	s.renderPublicCards(w, r)
}

func (s *Server) handlePublicClose(w http.ResponseWriter, r *http.Request) {
	target := r.FormValue("target")
	status, err := s.tunnel.Close(r.Context(), target)
	if err != nil {
		log.Printf("tunnel close %s: %v", target, err)
		s.triggerToast(w, "error", "Schließen fehlgeschlagen: "+err.Error())
	} else {
		s.gate.set(status.AnyOpen())
		s.triggerToast(w, "success", "Tunnel wird geschlossen.")
	}
	s.renderPublicCards(w, r)
}

func (s *Server) handlePublicCloseAll(w http.ResponseWriter, r *http.Request) {
	status, err := s.tunnel.CloseAll(r.Context())
	if err != nil {
		log.Printf("tunnel close-all: %v", err)
		s.triggerToast(w, "error", "Schließen fehlgeschlagen: "+err.Error())
	} else {
		s.gate.set(status.AnyOpen())
		s.triggerToast(w, "success", "Alles wird geschlossen.")
	}
	s.renderPublicCards(w, r)
}

func (s *Server) renderPublicCards(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = public.Cards(s.publicVM(r.Context())).Render(r.Context(), w)
}

// publicVM holt den Status und übersetzt ihn in die Anzeige. Die Umrechnung
// (Restzeit, Zustandstext) passiert hier, damit das Template nur noch ausgibt.
func (s *Server) publicVM(ctx context.Context) public.VM {
	if s.tunnel == nil {
		return public.VM{}
	}
	vm := public.VM{Configured: true}

	status, err := s.tunnel.Status(ctx)
	if err != nil {
		vm.Err = err.Error()
		return vm
	}
	s.gate.set(status.AnyOpen())

	now := time.Now()
	for _, t := range status.Targets {
		tv := public.TargetVM{
			Name:      t.Name,
			Label:     t.Label,
			State:     string(t.State),
			StateText: stateText(t.State),
			URL:       t.URL,
			Error:     t.Error,
			Busy:      t.State == tunnel.StateOpening || t.State == tunnel.StateClosing,
			Active:    t.State == tunnel.StateActive,
		}
		if t.Since != nil {
			tv.SinceText = "seit " + t.Since.Local().Format("15:04")
		}
		if t.ExpiresAt != nil && t.State == tunnel.StateActive {
			tv.ExpiresText = "schließt " + t.ExpiresAt.Local().Format("15:04") + " (noch " + humanRest(t.ExpiresAt.Sub(now)) + ")"
		}
		vm.Targets = append(vm.Targets, tv)
	}
	vm.TTLOptions = ttlOptions(status.DefaultTTLSeconds, status.MaxTTLSeconds)
	return vm
}

func stateText(state tunnel.State) string {
	switch state {
	case tunnel.StateActive:
		return "öffentlich erreichbar"
	case tunnel.StateOpening:
		return "baut auf …"
	case tunnel.StateClosing:
		return "baut ab …"
	case tunnel.StateError:
		return "Fehler"
	default:
		return "nicht erreichbar"
	}
}

// humanRest: "2 h 05 min" bzw. "12 min" — Sekunden interessieren nicht.
func humanRest(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < time.Hour {
		return fmt.Sprintf("%d min", int(d.Minutes()))
	}
	return fmt.Sprintf("%d h %02d min", int(d.Hours()), int(d.Minutes())%60)
}

// ttlOptions bietet 2/8/24 Stunden an, begrenzt auf das, was der
// tunnel-service überhaupt zulässt.
func ttlOptions(defaultSeconds, maxSeconds int) []public.TTLOption {
	var out []public.TTLOption
	for _, h := range []int{2, 8, 24} {
		if maxSeconds > 0 && h*3600 > maxSeconds {
			continue
		}
		out = append(out, public.TTLOption{
			Hours:    h,
			Label:    strconv.Itoa(h) + " h",
			Selected: h*3600 == defaultSeconds,
		})
	}
	if len(out) == 0 {
		out = append(out, public.TTLOption{Hours: 1, Label: "1 h", Selected: true})
	}
	if !anySelected(out) {
		out[0].Selected = true
	}
	return out
}

func anySelected(opts []public.TTLOption) bool {
	for _, o := range opts {
		if o.Selected {
			return true
		}
	}
	return false
}

// notifyMinInterval bremst den Versand aufs Handy. Zwei Klicks hintereinander
// sind ein Versehen, keine zwei Nachrichten.
const notifyMinInterval = 5 * time.Second

type notifyThrottle struct {
	mu   sync.Mutex
	last time.Time
}

// allow meldet, ob jetzt gesendet werden darf, und merkt sich den Zeitpunkt.
func (t *notifyThrottle) allow(now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.last.IsZero() && now.Sub(t.last) < notifyMinInterval {
		return false
	}
	t.last = now
	return true
}

// handlePublicNotify schickt die Adresse eines offenen Tunnels per WhatsApp an
// die eigene Nummer. Den Versand macht der whatsapp-bot (POST /notify) — er
// hat den Evolution-Zugang, und sein Empfänger steht fest auf PREVIEW_JID.
// Das Admin-UI kann darüber keine Gruppe erreichen.
func (s *Server) handlePublicNotify(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("target")

	status, err := s.tunnel.Status(r.Context())
	if err != nil {
		log.Printf("tunnel status: %v", err)
		s.triggerToast(w, "error", "tunnel-service nicht erreichbar.")
		s.renderPublicCards(w, r)
		return
	}
	s.gate.set(status.AnyOpen())

	var target tunnel.Target
	for _, t := range status.Targets {
		if t.Name == name {
			target = t
		}
	}
	if target.State != tunnel.StateActive || target.URL == "" {
		s.triggerToast(w, "error", "Nur offene Tunnel lassen sich verschicken.")
		s.renderPublicCards(w, r)
		return
	}
	if !s.notify.allow(time.Now()) {
		s.triggerToast(w, "error", "Gerade erst verschickt — kurz warten.")
		s.renderPublicCards(w, r)
		return
	}

	if err := s.sendNotify(r.Context(), notifyText(target)); err != nil {
		log.Printf("notify: %v", err)
		s.triggerToast(w, "error", "Versand fehlgeschlagen: "+err.Error())
		s.renderPublicCards(w, r)
		return
	}
	s.triggerToast(w, "success", "Adresse ist unterwegs aufs Handy.")
	s.renderPublicCards(w, r)
}

// notifyText baut die Nachricht. Kurz halten — sie landet auf dem Handy und
// soll dort vor allem einen antippbaren Link zeigen.
func notifyText(t tunnel.Target) string {
	msg := "🌍 " + t.Label + " ist öffentlich erreichbar:\n" + t.URL
	if t.ExpiresAt != nil {
		msg += "\n\nOffen bis " + t.ExpiresAt.Local().Format("15:04") + " Uhr."
	}
	return msg
}

func (s *Server) sendNotify(ctx context.Context, text string) error {
	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return err
	}
	url := strings.TrimRight(s.cfg.BotURL, "/") + "/notify"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := notifyClient.Do(req)
	if err != nil {
		return fmt.Errorf("Bot nicht erreichbar: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		msg := strings.TrimSpace(string(raw))
		if msg == "" {
			msg = resp.Status
		}
		return errors.New(msg)
	}
	return nil
}

var notifyClient = &http.Client{Timeout: 10 * time.Second}
