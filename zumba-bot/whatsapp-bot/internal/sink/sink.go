// Package sink stellt alternative Sender-Implementierungen für lokales Testen
// bereit: statt an die Evolution API zu senden, wird die erzeugte Nachricht
// nach stdout oder in eine Datei geschrieben. Implementiert dasselbe Interface
// wie evolution.Client (SendText).
package sink

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
)

func banner(number, text string) string {
	return fmt.Sprintf("\n===== WhatsApp → %s =====\n%s\n===== (Ende) =====\n", number, text)
}

// Writer schreibt jede Nachricht in einen io.Writer (z.B. os.Stdout).
type Writer struct {
	mu sync.Mutex
	w  io.Writer
}

func NewStdout() *Writer { return &Writer{w: os.Stdout} }

// NewWriter erlaubt ein beliebiges Ziel (z.B. ein Buffer in Tests).
func NewWriter(w io.Writer) *Writer { return &Writer{w: w} }

func (s *Writer) SendText(_ context.Context, number, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := io.WriteString(s.w, banner(number, text))
	return err
}

// File hängt jede Nachricht an eine Datei an.
type File struct {
	mu   sync.Mutex
	path string
}

func NewFile(path string) *File { return &File{path: path} }

func (f *File) SendText(_ context.Context, number, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	fh, err := os.OpenFile(f.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", f.path, err)
	}
	defer fh.Close()
	if _, err := io.WriteString(fh, banner(number, text)); err != nil {
		return fmt.Errorf("write %s: %w", f.path, err)
	}
	return nil
}
