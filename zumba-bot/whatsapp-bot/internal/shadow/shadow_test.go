package shadow

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
)

// fakeDB merkt sich den INSERT und ob dessen Kontext beim Aufruf noch lebte.
type fakeDB struct {
	query    string
	args     []any
	ctxErr   error
	affected int64
}

func (f *fakeDB) ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error) {
	f.query, f.args, f.ctxErr = q, args, ctx.Err()
	return fakeResult(f.affected), nil
}

type fakeResult int64

func (r fakeResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeResult) RowsAffected() (int64, error) { return int64(r), nil }

func newTestShadow(db *fakeDB) *Shadow {
	return &Shadow{db: db, LabelTimeout: 50 * time.Millisecond, InsertTimeout: time.Second}
}

var modelPred = &Prediction{Label: "invalid", Confidence: 0.57, Model: "NB-LogReg 0.2.0"}

// Hängt Gemini bis zum Timeout, wird die Zeile trotzdem geschrieben – mit
// eigenem, frischem Kontext und ohne Geminis Label (19.11.: der INSERT lief
// mit dem von Gemini aufgebrauchten Kontext und die Nachricht fehlte).
func TestGeminiHaengtZeileKommtTrotzdem(t *testing.T) {
	db := &fakeDB{affected: 1}
	s := newTestShadow(db)
	s.Gemini = func(ctx context.Context, _ string) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}
	err := s.record(Record{UserID: "u1", UserName: "Sepp", Message: "sag ab", DecidedBy: "modell", Model: modelPred})
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if db.query == "" {
		t.Fatal("kein INSERT")
	}
	if db.ctxErr != nil {
		t.Errorf("INSERT mit abgelaufenem Kontext: %v", db.ctxErr)
	}
	if g := db.args[3].(sql.NullString); g.Valid {
		t.Errorf("gemini_label = %q, want NULL", g.String)
	}
	if a := db.args[6].(sql.NullBool); a.Valid {
		t.Error("agree gesetzt, obwohl Geminis Label fehlt")
	}
}

func TestGeminiLabelWirdNachgeholt(t *testing.T) {
	db := &fakeDB{affected: 1}
	s := newTestShadow(db)
	s.Gemini = func(context.Context, string) (string, error) { return "false", nil }
	if err := s.record(Record{UserID: "u1", Message: "sag ab", DecidedBy: "modell", Model: modelPred}); err != nil {
		t.Fatalf("record: %v", err)
	}
	if g := db.args[3].(sql.NullString); g.String != "false" {
		t.Errorf("gemini_label = %+v", g)
	}
	if a := db.args[6].(sql.NullBool); !a.Valid || a.Bool {
		t.Errorf("agree = %+v, want false", a)
	}
	if strings.Contains(db.query, "NOT EXISTS") {
		t.Error("Live-Pfad soll ohne Doppel-Prüfung schreiben")
	}
}

// Nachverarbeitung: nur schreiben, wenn es Nutzer + Text an dem Tag noch
// nicht gibt – Zeitraum ist der ganze Tag in Ortszeit.
func TestNachverarbeitungPrueftDoppelte(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Berlin")
	day := time.Date(2026, 11, 19, 0, 0, 0, 0, loc)
	db := &fakeDB{affected: 0}
	s := newTestShadow(db)
	err := s.record(Record{UserID: "u1", Message: "sag ab", DecidedBy: "modell", Model: modelPred,
		GeminiLabel: "false", DedupeDay: day})
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if !strings.Contains(db.query, "NOT EXISTS") {
		t.Fatalf("ohne Doppel-Prüfung geschrieben: %s", db.query)
	}
	from, to := db.args[9].(time.Time), db.args[10].(time.Time)
	if !from.Equal(day) || !to.Equal(day.AddDate(0, 0, 1)) {
		t.Errorf("Zeitraum %v – %v", from, to)
	}
}
