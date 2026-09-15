// Package controller hält den Zustand der Tunnel: welches Ziel gerade offen
// ist, seit wann, bis wann — und schließt, was abgelaufen ist.
//
// Der Controller läuft als Sidecar neben dem ngrok-Agenten im selben Pod. Das
// ist Absicht: stirbt der Pod, sind Tunnel und Ablauffristen gleichzeitig weg.
// Ein Timer im Admin-UI hätte jeden Deploy überlebt, der Tunnel wäre offen
// geblieben.
package controller

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/michael/zumba-tunnel/internal/config"
	"github.com/michael/zumba-tunnel/internal/ngrok"
)

type State string

const (
	StateInactive State = "inactive"
	StateOpening  State = "opening"
	StateActive   State = "active"
	StateClosing  State = "closing"
	StateError    State = "error"
)

var (
	ErrUnknownTarget = errors.New("unbekanntes Ziel")
	// ErrBusy: es läuft gerade ein Auf- oder Abbau. Zwei gleichzeitige
	// Kommandos auf dasselbe Ziel wären ein Rennen gegen die ngrok-Cloud.
	ErrBusy = errors.New("Ziel ist gerade in Arbeit")
)

// Agent ist der Ausschnitt der ngrok-Agent-API, den der Controller braucht.
type Agent interface {
	List(ctx context.Context) (map[string]ngrok.Tunnel, error)
	Start(ctx context.Context, name, addr string) (string, error)
	Stop(ctx context.Context, name string) error
}

type Status struct {
	Name      string     `json:"name"`
	Label     string     `json:"label"`
	State     State      `json:"state"`
	URL       string     `json:"url,omitempty"`
	Since     *time.Time `json:"since,omitempty"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	Requests  int64      `json:"requests"`
	Error     string     `json:"error,omitempty"`
}

type entry struct {
	target   config.Target
	state    State
	url      string
	since    time.Time
	expires  time.Time
	requests int64
	errMsg   string

	// gen zählt jede Zustandsänderung. Ein asynchroner Start/Stop schreibt
	// sein Ergebnis nur, wenn die Generation noch stimmt — sonst hat
	// inzwischen jemand anders geschaltet und das Ergebnis ist veraltet.
	gen uint64
}

type Controller struct {
	mu      sync.Mutex
	agent   Agent
	order   []string
	entries map[string]*entry

	defaultTTL time.Duration
	maxTTL     time.Duration

	// adopted: beim ersten Abgleich übernehmen wir Tunnel, die der Agent schon
	// offen hat (der Controller kann neu gestartet sein, während der Agent
	// weiterlief). Danach gilt ein unbekannter Tunnel als Leiche und wird
	// geschlossen.
	adopted bool

	now func() time.Time
	run func(func()) // in Tests synchron
}

func New(agent Agent, cfg config.Config) *Controller {
	c := &Controller{
		agent:      agent,
		entries:    make(map[string]*entry, len(cfg.Targets)),
		defaultTTL: cfg.DefaultTTL,
		maxTTL:     cfg.MaxTTL,
		now:        time.Now,
		run:        func(f func()) { go f() },
	}
	for _, t := range cfg.Targets {
		c.order = append(c.order, t.Name)
		c.entries[t.Name] = &entry{target: t, state: StateInactive}
	}
	return c
}

// Open öffnet ein Ziel. Der Aufruf kehrt sofort zurück — der Aufbau läuft im
// Hintergrund, damit das Admin-UI "baut auf" anzeigen kann statt zu hängen.
func (c *Controller) Open(name string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.entries[name]
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownTarget, name)
	}
	switch e.state {
	case StateOpening, StateClosing:
		return ErrBusy
	case StateActive:
		// Schon offen: nur die Frist neu setzen. Zweimal Drücken soll die
		// Laufzeit verlängern, nicht scheitern.
		e.expires = c.now().Add(c.clamp(ttl))
		return nil
	}

	e.state = StateOpening
	e.errMsg = ""
	e.url = ""
	e.requests = 0
	e.gen++
	gen := e.gen
	addr := e.target.Addr
	deadline := c.clamp(ttl)

	c.run(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		url, err := c.agent.Start(ctx, name, addr)

		c.mu.Lock()
		defer c.mu.Unlock()
		if e.gen != gen {
			return // inzwischen wurde anders geschaltet
		}
		if err != nil {
			log.Printf("tunnel %s öffnen: %v", name, err)
			e.state = StateError
			e.errMsg = err.Error()
			return
		}
		now := c.now()
		e.state = StateActive
		e.url = url
		e.since = now
		e.expires = now.Add(deadline)
		log.Printf("tunnel %s offen: %s (bis %s)", name, url, e.expires.Format(time.RFC3339))
	})
	return nil
}

// Close schließt ein Ziel. Ein bereits geschlossenes Ziel ist kein Fehler.
func (c *Controller) Close(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[name]
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownTarget, name)
	}
	c.closeLocked(e)
	return nil
}

// CloseAll ist der Not-Aus (Knopf im Admin-UI und nächtlicher CronJob).
func (c *Controller) CloseAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, name := range c.order {
		c.closeLocked(c.entries[name])
	}
}

func (c *Controller) closeLocked(e *entry) {
	if e.state == StateInactive || e.state == StateClosing {
		return
	}
	name := e.target.Name
	e.state = StateClosing
	e.errMsg = ""
	e.gen++
	gen := e.gen

	c.run(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := c.agent.Stop(ctx, name)

		c.mu.Lock()
		defer c.mu.Unlock()
		if e.gen != gen {
			return
		}
		if err != nil {
			log.Printf("tunnel %s schließen: %v", name, err)
			// Fehler beim Schließen ist der unangenehme Fall: der Tunnel steht
			// womöglich noch. Als Fehler anzeigen, der Abgleich räumt nach.
			e.state = StateError
			e.errMsg = err.Error()
			return
		}
		e.state = StateInactive
		e.url = ""
		e.since = time.Time{}
		e.expires = time.Time{}
		log.Printf("tunnel %s geschlossen", name)
	})
}

// Status liefert alle Ziele in der konfigurierten Reihenfolge.
func (c *Controller) Status() []Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Status, 0, len(c.order))
	for _, name := range c.order {
		e := c.entries[name]
		s := Status{
			Name:     e.target.Name,
			Label:    e.target.Label,
			State:    e.state,
			URL:      e.url,
			Requests: e.requests,
			Error:    e.errMsg,
		}
		if !e.since.IsZero() {
			since := e.since
			s.Since = &since
		}
		if !e.expires.IsZero() {
			exp := e.expires
			s.ExpiresAt = &exp
		}
		out = append(out, s)
	}
	return out
}

// Run hält Zustand und Agent im Gleichtakt: abgelaufene Tunnel schließen,
// Abweichungen zwischen unserem Bild und dem Agenten ausgleichen.
func (c *Controller) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.Tick(ctx)
		}
	}
}

// Tick ist ein Durchlauf von Run (in Tests direkt aufrufbar).
func (c *Controller) Tick(ctx context.Context) {
	c.expire()
	c.reconcile(ctx)
}

func (c *Controller) expire() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	for _, name := range c.order {
		e := c.entries[name]
		if e.state == StateActive && !e.expires.IsZero() && !now.Before(e.expires) {
			log.Printf("tunnel %s: Laufzeit abgelaufen", name)
			c.closeLocked(e)
		}
	}
}

// reconcile gleicht mit dem Agenten ab. Der Agent ist die Wahrheit: was er
// nicht kennt, ist zu; was er kennt und wir nicht wollten, wird geschlossen.
func (c *Controller) reconcile(ctx context.Context) {
	listCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	live, err := c.agent.List(listCtx)
	if err != nil {
		log.Printf("Abgleich mit Agent: %v", err)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	adopting := !c.adopted
	c.adopted = true

	for _, name := range c.order {
		e := c.entries[name]
		t, open := live[name]
		delete(live, name)

		switch e.state {
		case StateOpening, StateClosing:
			continue // Ergebnis steht noch aus, nicht dazwischenfunken
		case StateActive:
			if !open {
				// Agent kennt den Tunnel nicht mehr (Neustart, Abbruch).
				log.Printf("tunnel %s ist beim Agenten verschwunden", name)
				e.state = StateInactive
				e.url = ""
				e.since = time.Time{}
				e.expires = time.Time{}
				continue
			}
			e.url = t.URL
			e.requests = t.Requests
		default:
			if !open {
				continue
			}
			if adopting {
				// Erster Durchlauf nach dem Start: der Agent lief weiter,
				// während dieser Container neu startete. Tunnel übernehmen —
				// mit frischer Frist, damit er trotzdem irgendwann zugeht.
				now := c.now()
				e.state = StateActive
				e.url = t.URL
				e.requests = t.Requests
				e.since = now
				e.expires = now.Add(c.defaultTTL)
				log.Printf("tunnel %s übernommen: %s", name, t.URL)
				continue
			}
			if e.state == StateError {
				// Ein Schließen war fehlgeschlagen und der Tunnel steht noch:
				// erneut versuchen, diesmal über den normalen Weg, damit der
				// Eintrag am Ende wieder sauber auf inactive steht.
				c.closeLocked(e)
				continue
			}
			// Kein Übernahmefall: ein Tunnel, den niemand wollte (z.B. ein
			// Start, den wir schon aufgegeben hatten). Der Eintrag ist bereits
			// inactive — hier ist nur beim Agenten aufzuräumen.
			log.Printf("tunnel %s war unerwartet offen — wird geschlossen", name)
			c.stopAsync(name)
		}
	}

	// Übrig bleiben Tunnel unter fremden Namen. Der Agent gehört uns allein,
	// also gehören sie da nicht hin.
	for name := range live {
		log.Printf("unbekannter tunnel %s beim Agenten — wird geschlossen", name)
		c.stopAsync(name)
	}
}

// stopAsync schließt einen Tunnel beim Agenten, ohne einen Eintrag zu führen —
// für Leichen und fremde Namen.
func (c *Controller) stopAsync(name string) {
	c.run(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := c.agent.Stop(ctx, name); err != nil {
			log.Printf("tunnel %s schließen: %v", name, err)
		}
	})
}

func (c *Controller) clamp(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return c.defaultTTL
	}
	if ttl > c.maxTTL {
		return c.maxTTL
	}
	return ttl
}
