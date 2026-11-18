package store

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// Der Lebenslauf einer Tisch-Erinnerung gegen eine echte DB: vorgenerieren,
// ablehnen, von Hand übernehmen, senden.
//
// Braucht eine Wegwerf-DB: ZUMBA_TEST_DSN=postgres://... go test ./store/
func TestWirtNachrichtLebenslauf(t *testing.T) {
	dsn := os.Getenv("ZUMBA_TEST_DSN")
	if dsn == "" {
		t.Skip("ZUMBA_TEST_DSN nicht gesetzt")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS wirt_settings, wirt_nachrichten`); err != nil {
		t.Fatal(err)
	}
	// Zweimal: das Anlegen muss idempotent sein.
	for i := 0; i < 2; i++ {
		if err := EnsureWirtSchema(ctx, db); err != nil {
			t.Fatal(err)
		}
	}

	// Einstellungen: ohne Zeile ist der Versand an und kein Modell gewählt.
	if s, err := GetWirtSettings(ctx, db); err != nil || !s.Versand || s.Model != "" {
		t.Fatalf("Startwert = %+v, %v", s, err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(SeedWirtModel(ctx, db, "start"))
	must(SetWirtVersand(ctx, db, false))
	must(SetWirtModel(ctx, db, "gewaehlt"))
	must(SeedWirtModel(ctx, db, "start")) // darf die Wahl nicht überschreiben
	if s, _ := GetWirtSettings(ctx, db); s.Versand || s.Model != "gewaehlt" || s.UpdatedAt.IsZero() {
		t.Fatalf("Einstellungen = %+v", s)
	}
	must(SetWirtVersand(ctx, db, true))
	if s, _ := GetWirtSettings(ctx, db); !s.Versand || s.Model != "gewaehlt" {
		t.Fatalf("Versand an: %+v", s)
	}

	tag := time.Date(2026, 11, 19, 0, 0, 0, 0, time.FixedZone("CET", 3600))
	get := func() *WirtNachricht {
		t.Helper()
		n, err := GetWirtNachricht(ctx, db, tag)
		must(err)
		return n
	}
	if get() != nil {
		t.Fatal("vor dem ersten Lauf darf es nichts geben")
	}

	// Fehlgeschlagene Vorgenerierung: Zeile ohne Text, mit Grund.
	must(SaveWirtFehler(ctx, db, tag, "Modell weg"))
	if n := get(); n == nil || n.Text != "" || n.Fehler != "Modell weg" || n.Verwendbar() {
		t.Fatalf("nach Fehler: %+v", n)
	}
	// Der nächste Versuch darf sie füllen.
	ok, err := SaveWirtEntwurf(ctx, db, tag, "erster {wirt}", "gemma", WirtQuelleAuto, false)
	must(err)
	if n := get(); !ok || n.Text != "erster {wirt}" || n.Fehler != "" || !n.Verwendbar() || n.Tag.Format("2006-01-02") != "2026-11-19" {
		t.Fatalf("nach Entwurf: ok=%v %+v", ok, n)
	}
	// Ein späterer Fehler ändert einen vorhandenen Text nicht.
	must(SaveWirtFehler(ctx, db, tag, "später"))
	if n := get(); n.Text != "erster {wirt}" || n.Fehler != "" {
		t.Fatalf("Fehler hat den Text berührt: %+v", n)
	}

	// Ablehnen – und ein Wiederholungslauf der Vorgenerierung hebt das nicht auf.
	if ok, err := SetWirtAbgelehnt(ctx, db, tag, true); err != nil || !ok {
		t.Fatalf("ablehnen: %v %v", ok, err)
	}
	if ok, _ := SaveWirtEntwurf(ctx, db, tag, "zweiter {wirt}", "gemma", WirtQuelleAuto, false); ok {
		t.Fatal("Vorgenerierung hat einen abgelehnten Text ersetzt")
	}
	if n := get(); n.Verwendbar() || n.AbgelehntAm == nil || n.Text != "erster {wirt}" {
		t.Fatalf("nach Ablehnung: %+v", n)
	}
	// Ablehnung zurücknehmen.
	if ok, _ := SetWirtAbgelehnt(ctx, db, tag, false); !ok || !get().Verwendbar() {
		t.Fatal("Ablehnung nicht zurückgenommen")
	}
	// Von Hand übernommen: ersetzt und gilt, auch nach einer Ablehnung.
	_, _ = SetWirtAbgelehnt(ctx, db, tag, true)
	if ok, _ := SaveWirtEntwurf(ctx, db, tag, "übernommen {wirt}", "flash", WirtQuelleTest, true); !ok {
		t.Fatal("Übernehmen hat nichts geschrieben")
	}
	if n := get(); !n.Verwendbar() || n.Text != "übernommen {wirt}" || n.Quelle != WirtQuelleTest || n.Model != "flash" {
		t.Fatalf("nach Übernehmen: %+v", n)
	}

	// Gesendet: danach ändert niemand mehr etwas.
	must(MarkWirtGesendet(ctx, db, tag, WirtAlsKI))
	if n := get(); n.GesendetAm == nil || n.GesendetAls != WirtAlsKI {
		t.Fatalf("nach Versand: %+v", n)
	}
	if ok, _ := SaveWirtEntwurf(ctx, db, tag, "zu spät {wirt}", "flash", WirtQuelleTest, true); ok {
		t.Fatal("Text nach dem Versand ersetzt")
	}
	if ok, _ := SetWirtAbgelehnt(ctx, db, tag, true); ok {
		t.Fatal("nach dem Versand abgelehnt")
	}
	if err := MarkWirtGesendet(ctx, db, tag, "quatsch"); err == nil {
		t.Fatal("unbekannte Variante angenommen")
	}

	// Ein Tag ohne jede Vorbereitung: der Versand legt die Zeile selbst an.
	vorwoche := tag.AddDate(0, 0, -7)
	must(MarkWirtGesendet(ctx, db, vorwoche, WirtAlsStandard))
	list, err := ListWirtNachrichten(ctx, db, 10)
	must(err)
	if len(list) != 2 || list[0].Text != "übernommen {wirt}" || list[1].GesendetAls != WirtAlsStandard || list[1].Text != "" {
		t.Fatalf("Liste = %+v", list)
	}
	if ok, _ := SetWirtAbgelehnt(ctx, db, vorwoche, true); ok {
		t.Fatal("Tag ohne Text abgelehnt")
	}
}
