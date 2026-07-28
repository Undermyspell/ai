// Package mldocs rendert die ML-Klassifikator-Doku (README aus ml-classifier/)
// als HTML-Seite im Admin-UI.
//
// README.md hier ist eine KOPIE — Quelle der Wahrheit ist
// ml-classifier/README.md. Nach Doku-Änderungen neu kopieren:
//
//	cp ../ml-classifier/README.md web/templates/mldocs/README.md
package mldocs

import (
	"bytes"
	_ "embed"
	"log"
	"sync"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

//go:embed README.md
var readme []byte

var (
	once sync.Once
	html string
)

// HTML liefert die gerenderte Doku (einmalig beim ersten Aufruf gerendert).
func HTML() string {
	once.Do(func() {
		md := goldmark.New(goldmark.WithExtensions(extension.GFM))
		var buf bytes.Buffer
		if err := md.Convert(readme, &buf); err != nil {
			log.Printf("mldocs: Markdown-Rendering fehlgeschlagen: %v", err)
			html = "<p>Doku konnte nicht gerendert werden.</p>"
			return
		}
		html = buf.String()
	})
	return html
}
