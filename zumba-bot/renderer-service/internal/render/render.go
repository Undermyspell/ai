// Package render schießt PNG-Screenshots von selbst mitgebrachtem HTML über
// headless Chromium (chromedp). Pro Aufruf startet ein frischer
// Chromium-Prozess und wird danach beendet — kein Idle-RAM, keine
// Zombie-Tabs; die ~1-2s Startzeit sind für Wochenreport/Bot-Test egal.
package render

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

// Renderer serialisiert Render-Aufrufe: mehr als ein Chromium gleichzeitig
// braucht der Anwendungsfall nicht, und es schont den RAM des Pi.
type Renderer struct {
	mu sync.Mutex
}

func New() *Renderer { return &Renderer{} }

// Options steuert Viewport-Breite und Skalierung (Retina-Faktor) des Shots.
type Options struct {
	Width int     // CSS-Pixel; Default 720
	Scale float64 // deviceScaleFactor; Default 2 (scharfe WhatsApp-Darstellung)
}

func (o Options) withDefaults() Options {
	if o.Width <= 0 {
		o.Width = 720
	}
	if o.Scale <= 0 {
		o.Scale = 2
	}
	return o
}

// PNG rendert das HTML-Dokument und liefert einen Full-Page-Screenshot:
// Breite = Options.Width, Höhe = gewachsene Dokumenthöhe.
func (r *Renderer) PNG(ctx context.Context, html string, opts Options) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	opts = opts.withDefaults()

	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoSandbox, // Container läuft ohne user namespaces
		chromedp.DisableGPU,
		chromedp.Flag("hide-scrollbars", true),
		chromedp.Flag("force-color-profile", "srgb"),
		chromedp.Flag("font-render-hinting", "none"),
	)
	if p := os.Getenv("CHROME_PATH"); p != "" {
		allocOpts = append(allocOpts, chromedp.ExecPath(p))
	}

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, allocOpts...)
	defer cancelAlloc()
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()
	runCtx, cancelRun := context.WithTimeout(browserCtx, 60*time.Second)
	defer cancelRun()

	dataURL := "data:text/html;charset=utf-8;base64," +
		base64.StdEncoding.EncodeToString([]byte(html))

	var buf []byte
	err := chromedp.Run(runCtx,
		// Viewport bewusst niedrig: FullScreenshot wächst auf die
		// Dokumenthöhe, so gibt es keinen Leerraum unter kurzen Karten.
		chromedp.EmulateViewport(int64(opts.Width), 200, chromedp.EmulateScale(opts.Scale)),
		chromedp.Navigate(dataURL),
		chromedp.WaitReady("body"),
		chromedp.FullScreenshot(&buf, 100),
	)
	if err != nil {
		return nil, fmt.Errorf("chromedp: %w", err)
	}
	return buf, nil
}
