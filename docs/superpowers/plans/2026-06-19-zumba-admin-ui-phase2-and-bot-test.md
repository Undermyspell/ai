# zumba-admin-ui Phase 2 (CRUD) + Bot-Test-Seite — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the missing write operations (absence + excluded-day CRUD) to `zumba-admin-ui`, plus a "Bot-Test" page that drives the `whatsapp-bot` via its new `/test` endpoint, plus Phase-3 Helm deployment files.

**Architecture:** The bot gains a `/test` endpoint that runs the existing workflow logic with guards bypassed and returns a structured `Outcome` JSON. The admin-ui gains write store methods, HTMX toggle/CRUD handlers with toast feedback, and a Bot-Test page that server-side-proxies to the bot. Deployment mirrors the existing n8n/whatsapp-bot Helm templates.

**Tech Stack:** Go 1.25.x, vanilla `net/http`, `a-h/templ`, HTMX (vendored), `lib/pq`, Postgres `zumba`. Two Go modules: `github.com/michael/zumba-whatsapp-bot` (`n8n/whatsapp-bot/`) and `github.com/michael/zumba-admin-ui` (`n8n/zumba-admin-ui/`).

## Global Constraints

- All user-facing strings are **German**. Do not translate UI/log text. Date order DD.MM, ISO week Monday, Thursday = `time.Thursday` / ISODOW 4.
- Only **Thursdays** are valid for absences and excluded days.
- `templ generate ./...` MUST run before `go build`/`go test` in `zumba-admin-ui` (the `*_templ.go` files are gitignored and generated). `make gen` does this.
- Do NOT edit `*_templ.go` files — edit the `.templ` source.
- Postgres writes target schema `public`, DB `zumba`, and assume a unique constraint on `stammtisch_abwesenheit ("userId", date)`.
- **No cluster actions**: write Helm files only; never `kubectl apply`, never create real SealedSecrets.
- Commit after each task. Work on a feature branch (not `main`).

---

## Part A — whatsapp-bot: `/test` endpoint

### Task 1 — A1: Refactor `process` → `run` returning `Outcome`, add `/test` route

**Files:**
- Modify: `n8n/whatsapp-bot/internal/web/server.go`
- Test: `n8n/whatsapp-bot/internal/web/server_test.go` (update existing), `n8n/whatsapp-bot/internal/web/test_endpoint_test.go` (new)

**Interfaces:**
- Produces: `Outcome` struct; `(*Server).run(ctx context.Context, ev evolution.WebhookEvent, bypassGuards bool) Outcome`; `POST /test` route returning `Outcome` JSON.
- Consumes: existing `store.Store`, `Classifier`, `Sender`, `classifier.Result` constants.

- [ ] **Step 1: Write failing tests for `run` with bypass**

Add `n8n/whatsapp-bot/internal/web/test_endpoint_test.go`:

```go
package web

import (
	"context"
	"testing"

	"github.com/michael/zumba-whatsapp-bot/internal/classifier"
)

func TestRunStatistikReturnsText(t *testing.T) {
	s, st, snd := newTestServer(classifier.Invalid, friday) // friday: guards would block, but statistik ignores guards
	out := s.run(context.Background(), groupMsg("statistik"), false)
	if out.Path != "statistik" {
		t.Fatalf("Path = %q, want statistik", out.Path)
	}
	if !st.statsCalled || !snd.called {
		t.Error("stats/send not invoked")
	}
	if out.Message == "" || out.Recipient != testGroup {
		t.Errorf("bad outcome: %+v", out)
	}
}

func TestRunClassifyBypassMarksAbsent(t *testing.T) {
	s, st, _ := newTestServer(classifier.Absage, friday) // not Thursday, but bypass=true
	out := s.run(context.Background(), groupMsg("bin raus"), true)
	if out.Path != "classify" || out.Classification != "false" || out.Action != "marked_absent" {
		t.Fatalf("bad outcome: %+v", out)
	}
	if st.absentUserID != "user-123" {
		t.Errorf("MarkAbsent not called: %+v", st)
	}
}

func TestRunClassifyZusageMarksPresent(t *testing.T) {
	s, st, _ := newTestServer(classifier.Zusage, friday)
	out := s.run(context.Background(), groupMsg("bin dabei"), true)
	if out.Action != "marked_present" || st.presentUserID != "user-123" {
		t.Fatalf("bad outcome/store: %+v / %+v", out, st)
	}
}

func TestRunGuardsBlockWithoutBypass(t *testing.T) {
	s, st, _ := newTestServer(classifier.Absage, friday)
	out := s.run(context.Background(), groupMsg("bin raus"), false)
	if out.Path != "ignored" || out.Reason == "" {
		t.Fatalf("expected ignored with reason: %+v", out)
	}
	if st.absentUserID != "" {
		t.Error("no DB write expected when guards block")
	}
}
```

> `friday`, `thursday`, `groupMsg`, `newTestServer`, `testGroup` are already
> defined in the existing `server_test.go` (same package) — reuse them, don't
> redefine.

- [ ] **Step 2: Run tests to verify they fail to compile/fail**

Run: `cd n8n/whatsapp-bot && go test ./internal/web/ -run TestRun -v`
Expected: FAIL — `s.run undefined`, `Outcome` undefined.

- [ ] **Step 3: Refactor `server.go` — replace `process`/`sendStats` with `run` + `statsText`, add `Outcome`**

In `n8n/whatsapp-bot/internal/web/server.go` replace the `process` and `sendStats` methods with:

```go
// Outcome beschreibt das Ergebnis eines Webhook-/Test-Durchlaufs.
type Outcome struct {
	Path           string `json:"path"`           // "statistik" | "classify" | "ignored"
	Classification string `json:"classification"` // "true"|"false"|"invalid"
	Action         string `json:"action"`         // "marked_absent"|"marked_present"|"none"
	Message        string `json:"message"`        // Statistik-Text bzw. Eingabe-Text
	Recipient      string `json:"recipient"`
	Date           string `json:"date"`
	UserID         string `json:"userId"`
	Reason         string `json:"reason"`
}

func (s *Server) run(ctx context.Context, ev evolution.WebhookEvent, bypassGuards bool) Outcome {
	msg := ev.Message()

	if strings.EqualFold(strings.TrimSpace(msg), "statistik") {
		text := s.statsText(ctx, ev.RemoteJid())
		return Outcome{Path: "statistik", Message: text, Recipient: ev.RemoteJid()}
	}

	if !bypassGuards {
		if ev.MessageType() != "conversation" || ev.RemoteJid() != s.groupJID || !s.isThursday() {
			return Outcome{Path: "ignored", Reason: "guard: messageType/group/Donnerstag nicht erfüllt"}
		}
	}

	res, err := s.classifier.Classify(ctx, msg)
	if err != nil {
		log.Printf("⚠️  classifier: %v (→ %s)", err, res)
	}
	userID := ev.UserID()
	today := s.today()
	out := Outcome{
		Path:           "classify",
		Classification: string(res),
		Action:         "none",
		Message:        msg,
		Recipient:      ev.RemoteJid(),
		UserID:         userID,
		Date:           today.Format("2006-01-02"),
	}
	switch res {
	case classifier.Absage:
		if err := s.store.MarkAbsent(ctx, userID, today, msg); err != nil {
			log.Printf("⚠️  MarkAbsent(%s): %v", userID, err)
		} else {
			out.Action = "marked_absent"
			log.Printf("📝 Absage: %s (%s)", ev.UserName(), userID)
		}
	case classifier.Zusage:
		if err := s.store.MarkPresent(ctx, userID, today); err != nil {
			log.Printf("⚠️  MarkPresent(%s): %v", userID, err)
		} else {
			out.Action = "marked_present"
			log.Printf("📝 Zusage: %s (%s)", ev.UserName(), userID)
		}
	}
	return out
}

// statsText baut den Ranglisten-Text, sendet ihn über den Sender und gibt ihn zurück.
func (s *Server) statsText(ctx context.Context, receiver string) string {
	stats, err := s.store.UserStats(ctx)
	if err != nil {
		log.Printf("⚠️  UserStats: %v", err)
		return ""
	}
	text := report.Build(stats)
	if err := s.sender.SendText(ctx, receiver, text); err != nil {
		log.Printf("⚠️  SendText(%s): %v", receiver, err)
	} else {
		log.Printf("📊 Statistik gesendet an %s", receiver)
	}
	return text
}
```

Update `handleWebhook` to call `run`:

```go
func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	var ev evolution.WebhookEvent
	if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
		log.Printf("⚠️  webhook: invalid JSON: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	s.run(r.Context(), ev, false)
	w.WriteHeader(http.StatusOK)
}
```

Add a `/test` handler and register it in `Routes()`:

```go
func (s *Server) handleTest(w http.ResponseWriter, r *http.Request) {
	var ev evolution.WebhookEvent
	if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	out := s.run(r.Context(), ev, true)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
```

In `Routes()` add after the webhook line:

```go
	mux.HandleFunc("POST /test", s.handleTest)
```

- [ ] **Step 4: Update existing `server_test.go` to use `run`**

In `n8n/whatsapp-bot/internal/web/server_test.go`, replace each `s.process(context.Background(), ev)` call with `s.run(context.Background(), ev, false)` and (where the test only checked side effects) keep the store assertions. For `TestStatistikSendsStats` the call becomes `s.run(context.Background(), groupMsg("Statistik"), false)`.

- [ ] **Step 5: Run all bot tests**

Run: `cd n8n/whatsapp-bot && gofmt -w ./internal/web/ && go vet ./... && go test ./...`
Expected: PASS for all packages.

- [ ] **Step 6: Commit**

```bash
git add n8n/whatsapp-bot/internal/web/
git commit -m "feat(whatsapp-bot): add /test endpoint returning structured Outcome"
```

---

## Part B — admin-ui: Phase 2 write operations

### Task 2 — B1: Store write methods (interface + mock + postgres)

**Files:**
- Modify: `n8n/zumba-admin-ui/internal/store/store.go`, `internal/store/mock.go`, `internal/store/postgres.go`
- Test: `n8n/zumba-admin-ui/internal/store/mock_test.go` (new)

**Interfaces:**
- Produces (added to `store.Store`):
  - `InsertAbsence(ctx context.Context, userID string, date time.Time, message *string) error`
  - `DeleteAbsence(ctx context.Context, userID string, date time.Time) error`
  - `InsertExcludedDay(ctx context.Context, date time.Time) error`
  - `DeleteExcludedDay(ctx context.Context, date time.Time) error`

- [ ] **Step 1: Write failing mock tests**

Create `n8n/zumba-admin-ui/internal/store/mock_test.go`:

```go
package store

import (
	"context"
	"testing"
	"time"

	"github.com/michael/zumba-admin-ui/internal/timeutil"
)

func thursdayIn(p timeutil.Period) time.Time {
	for d := p.Start; !d.After(p.End); d = d.AddDate(0, 0, 1) {
		if d.Weekday() == time.Thursday {
			return d
		}
	}
	return p.Start
}

func TestMockInsertDeleteAbsence(t *testing.T) {
	p := timeutil.Period{Start: mustDate("2025-12-01"), End: mustDate("2026-11-30")}
	m := NewMock(p)
	ctx := context.Background()
	day := thursdayIn(p)
	uid := m.users[0].ID

	before := countAbsences(ctx, t, m, p, uid, day)
	msg := "bin raus"
	if err := m.InsertAbsence(ctx, uid, day, &msg); err != nil {
		t.Fatal(err)
	}
	if got := countAbsences(ctx, t, m, p, uid, day); got != 1 {
		t.Fatalf("after insert = %d, want 1 (before %d)", got, before)
	}
	// idempotent upsert
	if err := m.InsertAbsence(ctx, uid, day, &msg); err != nil {
		t.Fatal(err)
	}
	if got := countAbsences(ctx, t, m, p, uid, day); got != 1 {
		t.Fatalf("after re-insert = %d, want 1", got)
	}
	if err := m.DeleteAbsence(ctx, uid, day); err != nil {
		t.Fatal(err)
	}
	if got := countAbsences(ctx, t, m, p, uid, day); got != 0 {
		t.Fatalf("after delete = %d, want 0", got)
	}
}

func TestMockInsertDeleteExcluded(t *testing.T) {
	p := timeutil.Period{Start: mustDate("2025-12-01"), End: mustDate("2026-11-30")}
	m := NewMock(p)
	ctx := context.Background()
	day := thursdayIn(p)

	if err := m.InsertExcludedDay(ctx, day); err != nil {
		t.Fatal(err)
	}
	days, _ := m.ListExcludedDays(ctx, p)
	if !containsDate(days, day) {
		t.Fatal("excluded day not added")
	}
	if err := m.DeleteExcludedDay(ctx, day); err != nil {
		t.Fatal(err)
	}
	days, _ = m.ListExcludedDays(ctx, p)
	if containsDate(days, day) {
		t.Fatal("excluded day not removed")
	}
}

func countAbsences(ctx context.Context, t *testing.T, m *Mock, p timeutil.Period, uid string, day time.Time) int {
	t.Helper()
	all, err := m.ListAbsences(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, a := range all {
		if a.UserID == uid && timeutil.FormatISO(a.Date) == timeutil.FormatISO(day) {
			n++
		}
	}
	return n
}

func containsDate(days []time.Time, d time.Time) bool {
	for _, x := range days {
		if timeutil.FormatISO(x) == timeutil.FormatISO(d) {
			return true
		}
	}
	return false
}

func mustDate(s string) time.Time {
	d, err := timeutil.ParseISO(s)
	if err != nil {
		panic(err)
	}
	return d
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `cd n8n/zumba-admin-ui && go test ./internal/store/ -run TestMock -v`
Expected: FAIL — methods undefined.

- [ ] **Step 3: Extend the `Store` interface**

In `internal/store/store.go`, replace the comment block + interface end with the writer methods added:

```go
// Store is the interface used by handlers. Phase 2 adds the write methods.
type Store interface {
	ListUsers(ctx context.Context) ([]User, error)
	ListThursdays(ctx context.Context, p timeutil.Period) ([]time.Time, error)
	ListExcludedDays(ctx context.Context, p timeutil.Period) ([]time.Time, error)
	ListAbsences(ctx context.Context, p timeutil.Period) ([]Absence, error)
	Leaderboard(ctx context.Context, p timeutil.Period) ([]LeaderboardRow, error)

	InsertAbsence(ctx context.Context, userID string, date time.Time, message *string) error
	DeleteAbsence(ctx context.Context, userID string, date time.Time) error
	InsertExcludedDay(ctx context.Context, date time.Time) error
	DeleteExcludedDay(ctx context.Context, date time.Time) error
}
```

- [ ] **Step 4: Implement on Mock**

Append to `internal/store/mock.go`:

```go
func (m *Mock) InsertAbsence(_ context.Context, userID string, date time.Time, message *string) error {
	day := timeutil.StartOfDay(date)
	for i := range m.absences {
		if m.absences[i].UserID == userID && timeutil.FormatISO(m.absences[i].Date) == timeutil.FormatISO(day) {
			m.absences[i].Message = message // upsert
			return nil
		}
	}
	m.absences = append(m.absences, Absence{UserID: userID, Date: day, Message: message})
	return nil
}

func (m *Mock) DeleteAbsence(_ context.Context, userID string, date time.Time) error {
	out := m.absences[:0]
	for _, a := range m.absences {
		if a.UserID == userID && timeutil.FormatISO(a.Date) == timeutil.FormatISO(date) {
			continue
		}
		out = append(out, a)
	}
	m.absences = out
	return nil
}

func (m *Mock) InsertExcludedDay(_ context.Context, date time.Time) error {
	day := timeutil.StartOfDay(date)
	for _, d := range m.excludedDays {
		if timeutil.FormatISO(d) == timeutil.FormatISO(day) {
			return nil
		}
	}
	m.excludedDays = append(m.excludedDays, day)
	return nil
}

func (m *Mock) DeleteExcludedDay(_ context.Context, date time.Time) error {
	out := m.excludedDays[:0]
	for _, d := range m.excludedDays {
		if timeutil.FormatISO(d) == timeutil.FormatISO(date) {
			continue
		}
		out = append(out, d)
	}
	m.excludedDays = out
	return nil
}
```

- [ ] **Step 5: Implement on Postgres**

Append to `internal/store/postgres.go`:

```go
func (s *Postgres) InsertAbsence(ctx context.Context, userID string, date time.Time, message *string) error {
	const q = `
		INSERT INTO public.stammtisch_abwesenheit ("userId", date, message)
		VALUES ($1, $2, $3)
		ON CONFLICT ("userId", date) DO UPDATE SET message = EXCLUDED.message`
	if _, err := s.db.ExecContext(ctx, q, userID, date, message); err != nil {
		return fmt.Errorf("InsertAbsence: %w", err)
	}
	return nil
}

func (s *Postgres) DeleteAbsence(ctx context.Context, userID string, date time.Time) error {
	const q = `DELETE FROM public.stammtisch_abwesenheit WHERE "userId" = $1 AND date = $2`
	if _, err := s.db.ExecContext(ctx, q, userID, date); err != nil {
		return fmt.Errorf("DeleteAbsence: %w", err)
	}
	return nil
}

func (s *Postgres) InsertExcludedDay(ctx context.Context, date time.Time) error {
	const q = `INSERT INTO excluded_days (date) VALUES ($1) ON CONFLICT (date) DO NOTHING`
	if _, err := s.db.ExecContext(ctx, q, date); err != nil {
		return fmt.Errorf("InsertExcludedDay: %w", err)
	}
	return nil
}

func (s *Postgres) DeleteExcludedDay(ctx context.Context, date time.Time) error {
	const q = `DELETE FROM excluded_days WHERE date = $1`
	if _, err := s.db.ExecContext(ctx, q, date); err != nil {
		return fmt.Errorf("DeleteExcludedDay: %w", err)
	}
	return nil
}
```

> If `excluded_days` has no unique constraint on `date`, the `ON CONFLICT (date)` will error. Verify with `\d excluded_days`; if missing, the InsertExcludedDay handler (Task B3) guards against duplicates by checking the list first — but prefer adding the constraint. (Do not run DDL against the cluster as part of this plan; note it for the operator.)

- [ ] **Step 6: Run tests (rename helper to ASCII first)**

Run: `cd n8n/zumba-admin-ui && gofmt -w ./internal/store/ && go test ./internal/store/ -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add n8n/zumba-admin-ui/internal/store/
git commit -m "feat(admin-ui): add absence + excluded-day write store methods"
```

---

### Task 3 — B2: Toast infrastructure (HX-Trigger + JS)

**Files:**
- Create: `n8n/zumba-admin-ui/assets/static/js/toast.js`
- Modify: `n8n/zumba-admin-ui/web/templates/layout.templ` (include script), `internal/web/server.go` (add `triggerToast` helper)

**Interfaces:**
- Produces: `(*Server).triggerToast(w http.ResponseWriter, level, msg string)` sets an `HX-Trigger` header; client JS renders the toast into `#toast-stack`.

- [ ] **Step 1: Read the existing toast component + layout**

Read `web/templates/partials/toast.templ` and `web/templates/layout.templ` to match markup/classes (`#toast-stack`). The JS must produce the same DOM the `Toast()` component uses.

- [ ] **Step 2: Add the toast JS**

Create `assets/static/js/toast.js`:

```js
// Renders toasts triggered by the server via the HX-Trigger header:
//   HX-Trigger: {"showToast":{"level":"success|error","msg":"..."}}
document.body.addEventListener("showToast", function (e) {
  var d = e.detail || {};
  var stack = document.getElementById("toast-stack");
  if (!stack) return;
  var el = document.createElement("div");
  el.className = "toast toast-" + (d.level || "info");
  el.textContent = d.msg || "";
  stack.appendChild(el);
  setTimeout(function () {
    el.classList.add("toast-out");
    setTimeout(function () { el.remove(); }, 300);
  }, 3200);
});
```

- [ ] **Step 3: Include the script in the layout**

In `web/templates/layout.templ`, next to the existing `htmx.min.js` script tag, add:

```html
<script src="/static/js/toast.js" defer></script>
```

- [ ] **Step 4: Add `triggerToast` helper to the server**

In `internal/web/server.go` add:

```go
func (s *Server) triggerToast(w http.ResponseWriter, level, msg string) {
	// JSON object form of HX-Trigger so the client receives event detail.
	payload := fmt.Sprintf(`{"showToast":{"level":%q,"msg":%q}}`, level, msg)
	w.Header().Set("HX-Trigger", payload)
}
```

Add `"fmt"` to the import block if not present.

- [ ] **Step 5: Verify build (templ + go)**

Run: `cd n8n/zumba-admin-ui && make gen && go build ./...`
Expected: builds cleanly. (No unit test here; exercised via B3 handler tests.)

- [ ] **Step 6: Commit**

```bash
git add n8n/zumba-admin-ui/assets/static/js/toast.js n8n/zumba-admin-ui/web/templates/layout.templ n8n/zumba-admin-ui/internal/web/server.go
git commit -m "feat(admin-ui): wire toast notifications via HX-Trigger"
```

---

### Task 4 — B3: Excluded-day add/delete (handlers + routes + template)

**Files:**
- Modify: `internal/web/server.go` (routes + handlers), `web/templates/excluded/list.templ`
- Test: `internal/web/excluded_test.go` (new)

**Interfaces:**
- Consumes: `store.InsertExcludedDay`, `store.DeleteExcludedDay`, `timeutil.IsThursday`, `timeutil.ParseISO`, `triggerToast`.
- Produces: `POST /excluded` (form `date`), `DELETE /excluded/{date}`.

- [ ] **Step 1: Write failing handler tests**

Create `internal/web/excluded_test.go`:

```go
package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/michael/zumba-admin-ui/internal/config"
)

func TestPostExcludedThursday(t *testing.T) {
	spy := newSpyStore()
	srv := New(spy, testCfg(), false)
	form := url.Values{"date": {"2026-01-01"}} // Thursday
	req := httptest.NewRequest("POST", "/excluded", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	if spy.insertedExcluded != "2026-01-01" {
		t.Errorf("InsertExcludedDay not called with Thursday, got %q", spy.insertedExcluded)
	}
}

func TestPostExcludedRejectsNonThursday(t *testing.T) {
	spy := newSpyStore()
	srv := New(spy, testCfg(), false)
	form := url.Values{"date": {"2026-01-02"}} // Friday
	req := httptest.NewRequest("POST", "/excluded", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("code = %d, want 422", rec.Code)
	}
	if spy.insertedExcluded != "" {
		t.Error("InsertExcludedDay must not be called for non-Thursday")
	}
}

func TestDeleteExcluded(t *testing.T) {
	spy := newSpyStore()
	srv := New(spy, testCfg(), false)
	req := httptest.NewRequest("DELETE", "/excluded/2026-01-01", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	if spy.deletedExcluded != "2026-01-01" {
		t.Errorf("DeleteExcludedDay not called, got %q", spy.deletedExcluded)
	}
}

func testCfg() config.Config {
	return config.Config{
		EvalPeriodStart: mustDate("2025-12-01"),
		EvalPeriodEnd:   mustDate("2026-11-30"),
	}
}
```

Create the shared spy store `internal/web/spystore_test.go`:

```go
package web

import (
	"context"
	"time"

	"github.com/michael/zumba-admin-ui/internal/store"
	"github.com/michael/zumba-admin-ui/internal/timeutil"
)

// spyStore implements store.Store: reads return empty/minimal data, writes are recorded.
type spyStore struct {
	insertedAbsence  string // "userId@date"
	deletedAbsence   string
	insertedExcluded string // "YYYY-MM-DD"
	deletedExcluded  string
	users            []store.User
	absences         []store.Absence
}

func newSpyStore() *spyStore {
	return &spyStore{users: []store.User{{ID: "u01", Name: "Max"}}}
}

func (s *spyStore) ListUsers(context.Context) ([]store.User, error) { return s.users, nil }
func (s *spyStore) ListThursdays(_ context.Context, _ timeutil.Period) ([]time.Time, error) {
	return []time.Time{mustDate("2026-01-01")}, nil
}
func (s *spyStore) ListExcludedDays(_ context.Context, _ timeutil.Period) ([]time.Time, error) {
	return nil, nil
}
func (s *spyStore) ListAbsences(_ context.Context, _ timeutil.Period) ([]store.Absence, error) {
	return s.absences, nil
}
func (s *spyStore) Leaderboard(_ context.Context, _ timeutil.Period) ([]store.LeaderboardRow, error) {
	return nil, nil
}
func (s *spyStore) InsertAbsence(_ context.Context, userID string, date time.Time, _ *string) error {
	s.insertedAbsence = userID + "@" + timeutil.FormatISO(date)
	s.absences = append(s.absences, store.Absence{UserID: userID, Date: date})
	return nil
}
func (s *spyStore) DeleteAbsence(_ context.Context, userID string, date time.Time) error {
	s.deletedAbsence = userID + "@" + timeutil.FormatISO(date)
	return nil
}
func (s *spyStore) InsertExcludedDay(_ context.Context, date time.Time) error {
	s.insertedExcluded = timeutil.FormatISO(date)
	return nil
}
func (s *spyStore) DeleteExcludedDay(_ context.Context, date time.Time) error {
	s.deletedExcluded = timeutil.FormatISO(date)
	return nil
}

func mustDate(s string) time.Time {
	d, err := timeutil.ParseISO(s)
	if err != nil {
		panic(err)
	}
	return d
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `cd n8n/zumba-admin-ui && make gen && go test ./internal/web/ -run Excluded -v`
Expected: FAIL — routes return 404 (not registered) / spy fields empty.

- [ ] **Step 3: Register routes**

In `Routes()` (`internal/web/server.go`), add:

```go
	mux.HandleFunc("POST /excluded", s.handleAddExcluded)
	mux.HandleFunc("DELETE /excluded/{date}", s.handleDeleteExcluded)
```

- [ ] **Step 4: Implement handlers**

Add to `internal/web/server.go`:

```go
func (s *Server) handleAddExcluded(w http.ResponseWriter, r *http.Request) {
	date, err := timeutil.ParseISO(r.FormValue("date"))
	if err != nil {
		s.triggerToast(w, "error", "Ungültiges Datum.")
		http.Error(w, "ungültiges Datum", http.StatusUnprocessableEntity)
		return
	}
	if !timeutil.IsThursday(date) {
		s.triggerToast(w, "error", "Nur Donnerstage können gesperrt werden.")
		http.Error(w, "kein Donnerstag", http.StatusUnprocessableEntity)
		return
	}
	if err := s.store.InsertExcludedDay(r.Context(), date); err != nil {
		s.fail(w, "insert excluded", err)
		return
	}
	s.triggerToast(w, "success", "Sperrtag angelegt.")
	s.renderExcludedList(w, r)
}

func (s *Server) handleDeleteExcluded(w http.ResponseWriter, r *http.Request) {
	date, err := timeutil.ParseISO(r.PathValue("date"))
	if err != nil {
		http.Error(w, "ungültiges Datum", http.StatusUnprocessableEntity)
		return
	}
	if err := s.store.DeleteExcludedDay(r.Context(), date); err != nil {
		s.fail(w, "delete excluded", err)
		return
	}
	s.triggerToast(w, "success", "Sperrtag entfernt.")
	s.renderExcludedList(w, r)
}

// renderExcludedList renders just the list region (HTMX swap target).
func (s *Server) renderExcludedList(w http.ResponseWriter, r *http.Request) {
	all, err := s.store.ListExcludedDays(r.Context(), s.period())
	if err != nil {
		s.fail(w, "excluded", err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := excluded.ListRegion(excluded.ListVM{Days: all}).Render(r.Context(), w); err != nil {
		log.Printf("render excluded region: %v", err)
	}
}
```

- [ ] **Step 5: Update the excluded template — add form + delete + swap region**

Edit `web/templates/excluded/list.templ`. Wrap the list in a swap target and add a create form + per-row delete. Replace the `templ List` body and `row` with:

```go
templ List(vm ListVM) {
	<div class="page-header enter">
		<div class="eyebrow">Sperrtage</div>
		<h1>Ausgeschlossene Donnerstage</h1>
		<p class="meta">Diese Tage zählen nicht in der Auswertung.</p>
	</div>
	<form class="excluded-form enter" hx-post="/excluded" hx-target="#excluded-region" hx-swap="outerHTML">
		<input type="date" name="date" required aria-label="Donnerstag wählen"/>
		<button type="submit" class="btn-primary">Sperrtag anlegen</button>
	</form>
	@ListRegion(vm)
}

templ ListRegion(vm ListVM) {
	<div id="excluded-region">
		if len(vm.Days) == 0 {
			<div class="empty">
				<div class="icon">📭</div>
				<p>Keine Sperrtage in der aktuellen Saison.</p>
			</div>
		} else {
			<div class="list enter">
				for _, d := range vm.Days {
					@row(d)
				}
			</div>
		}
	</div>
}

templ row(d time.Time) {
	<div class="excluded-row">
		<span class="marker"></span>
		<div>
			<div class="label">{ timeutil.FormatDE(d) }</div>
			<div class="iso">{ fmt.Sprintf("KW %d · %s", isoWeek(d), timeutil.FormatISO(d)) }</div>
		</div>
		<button
			class="btn-danger"
			hx-delete={ "/excluded/" + timeutil.FormatISO(d) }
			hx-target="#excluded-region"
			hx-swap="outerHTML"
			hx-confirm="Sperrtag wirklich entfernen?"
		>Entfernen</button>
	</div>
}
```

- [ ] **Step 6: Run tests**

Run: `cd n8n/zumba-admin-ui && make gen && gofmt -w ./internal/web/ && go test ./internal/web/ -run Excluded -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add n8n/zumba-admin-ui/internal/web/ n8n/zumba-admin-ui/web/templates/excluded/
git commit -m "feat(admin-ui): excluded-day create/delete with HTMX + Thursday validation"
```

---

### Task 5 — B4: Absence toggle (handler + route + reusable partial)

**Files:**
- Create: `web/templates/partials/absence_toggle.templ`
- Modify: `internal/web/server.go` (route + handler), `web/templates/days/detail.templ` (use toggle), `web/templates/members/detail.templ` (use toggle)
- Test: `internal/web/toggle_test.go` (new)

**Interfaces:**
- Consumes: `store.InsertAbsence`, `store.DeleteAbsence`, `store.ListAbsences`, spy store.
- Produces: `POST /toggle-absence` (form `userId`, `date`); `partials.AbsenceToggle(userID string, date time.Time, absent bool)` component.

- [ ] **Step 1: Write failing tests**

Create `internal/web/toggle_test.go`:

```go
package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/michael/zumba-admin-ui/internal/store"
)

func postToggle(srv *Server, userID, date string) *httptest.ResponseRecorder {
	form := url.Values{"userId": {userID}, "date": {date}}
	req := httptest.NewRequest("POST", "/toggle-absence", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	return rec
}

func TestToggleMarksAbsentWhenPresent(t *testing.T) {
	spy := newSpyStore() // no absences → currently present
	srv := New(spy, testCfg(), false)
	rec := postToggle(srv, "u01", "2026-01-01")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	if spy.insertedAbsence != "u01@2026-01-01" {
		t.Errorf("expected InsertAbsence, got insert=%q delete=%q", spy.insertedAbsence, spy.deletedAbsence)
	}
}

func TestToggleMarksPresentWhenAbsent(t *testing.T) {
	spy := newSpyStore()
	spy.absences = []store.Absence{{UserID: "u01", Date: mustDate("2026-01-01")}}
	srv := New(spy, testCfg(), false)
	rec := postToggle(srv, "u01", "2026-01-01")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	if spy.deletedAbsence != "u01@2026-01-01" {
		t.Errorf("expected DeleteAbsence, got insert=%q delete=%q", spy.insertedAbsence, spy.deletedAbsence)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `cd n8n/zumba-admin-ui && make gen && go test ./internal/web/ -run Toggle -v`
Expected: FAIL — route 404.

- [ ] **Step 3: Create the reusable toggle partial**

Create `web/templates/partials/absence_toggle.templ`:

```go
package partials

import (
	"time"

	"github.com/michael/zumba-admin-ui/internal/timeutil"
)

// AbsenceToggle renders a single button that flips a user's attendance for a
// Thursday. It posts to /toggle-absence and swaps itself with the server's
// re-rendered response.
templ AbsenceToggle(userID string, date time.Time, absent bool) {
	<button
		class={ "toggle", templ.KV("is-absent", absent), templ.KV("is-present", !absent) }
		hx-post="/toggle-absence"
		hx-vals={ `{"userId":"` + userID + `","date":"` + timeutil.FormatISO(date) + `"}` }
		hx-swap="outerHTML"
	>
		if absent {
			Abgemeldet
		} else {
			Anwesend
		}
	</button>
}
```

- [ ] **Step 4: Register route + handler**

In `Routes()` add:

```go
	mux.HandleFunc("POST /toggle-absence", s.handleToggleAbsence)
```

Add handler to `internal/web/server.go`:

```go
func (s *Server) handleToggleAbsence(w http.ResponseWriter, r *http.Request) {
	userID := r.FormValue("userId")
	date, err := timeutil.ParseISO(r.FormValue("date"))
	if err != nil {
		http.Error(w, "ungültiges Datum", http.StatusUnprocessableEntity)
		return
	}
	if userID == "" {
		http.Error(w, "userId fehlt", http.StatusUnprocessableEntity)
		return
	}

	// Determine current state: is there an absence for this user+date?
	absences, err := s.store.ListAbsences(r.Context(), s.period())
	if err != nil {
		s.fail(w, "absences", err)
		return
	}
	currentlyAbsent := false
	iso := timeutil.FormatISO(date)
	for _, a := range absences {
		if a.UserID == userID && timeutil.FormatISO(a.Date) == iso {
			currentlyAbsent = true
			break
		}
	}

	var nowAbsent bool
	if currentlyAbsent {
		if err := s.store.DeleteAbsence(r.Context(), userID, date); err != nil {
			s.fail(w, "delete absence", err)
			return
		}
		nowAbsent = false
		s.triggerToast(w, "success", "Als anwesend markiert.")
	} else {
		if err := s.store.InsertAbsence(r.Context(), userID, date, nil); err != nil {
			s.fail(w, "insert absence", err)
			return
		}
		nowAbsent = true
		s.triggerToast(w, "success", "Als abgemeldet markiert.")
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := partials.AbsenceToggle(userID, date, nowAbsent).Render(r.Context(), w); err != nil {
		log.Printf("render toggle: %v", err)
	}
}
```

- [ ] **Step 5: Use the toggle in day + member detail templates**

In `web/templates/days/detail.templ`, inside `cellRow`, replace the static status `<span>` block:

```go
			if c.Absent {
				<span class="status">Abgemeldet</span>
			} else {
				<span class="status">Anwesend</span>
			}
```

with the interactive toggle (add the import `"github.com/michael/zumba-admin-ui/web/templates/partials"` and pass the day date into the cell). Since `cellRow` lacks the date, change `Detail` to call `@cellRow(vm.Date, c)` and update `cellRow`'s signature:

```go
templ Detail(vm DetailVM) {
	// ... unchanged header ...
	if !vm.Excluded {
		<div class="attendance-grid enter">
			for _, c := range vm.Cells {
				@cellRow(vm.Date, c)
			}
		</div>
	}
}

templ cellRow(date time.Time, c Cell) {
	<div class={ "attendance-cell", cellClass(c.Absent) }>
		<span class="emoji">{ emoji.For(c.Name) }</span>
		<div>
			<div class="name">{ c.Name }</div>
			if c.Absent && c.Message != nil && *c.Message != "" {
				<div class="msg">„{ *c.Message }"</div>
			}
		</div>
		@partials.AbsenceToggle(c.UserID, date, c.Absent)
	</div>
}
```

In `web/templates/members/detail.templ` (read it first), find where each Thursday entry renders its absent/present status and replace that status element with `@partials.AbsenceToggle(vm.User.ID, e.Date, e.Absent)` (the `members.DetailEntry` has `Date` and `Absent` per `server.go:202`). Add the `partials` import.

- [ ] **Step 6: Run tests + build**

Run: `cd n8n/zumba-admin-ui && make gen && gofmt -w ./internal/web/ && go test ./internal/web/ -run Toggle -v && go build ./...`
Expected: PASS + clean build.

- [ ] **Step 7: Commit**

```bash
git add n8n/zumba-admin-ui/internal/web/ n8n/zumba-admin-ui/web/templates/
git commit -m "feat(admin-ui): toggle attendance on day + member detail via HTMX"
```

---

## Part C — admin-ui: Bot-Test page

### Task 6 — C1: Config `BOT_URL`, embedded examples, page + example loader

**Files:**
- Modify: `internal/config/config.go`, `internal/web/server.go` (Server struct already holds cfg)
- Create: `web/bottest/examples/{zusage,absage,statistik}.json`, `web/bottest/embed.go`, `web/templates/bottest/page.templ`
- Modify: `web/templates/partials/nav.templ`

**Interfaces:**
- Produces: `cfg.BotURL`; `GET /bot-test`; `GET /bot-test/example/{kind}`; `bottest.Examples` (embed.FS); `bottest.Page(vm bottest.PageVM)` component.

- [ ] **Step 1: Add `BOT_URL` to config**

In `internal/config/config.go`, add field `BotURL string` to `Config` and in `Load()`:

```go
		BotURL: getenv("BOT_URL", "http://localhost:8080"),
```

(Place the field next to `Port`.)

- [ ] **Step 2: Add example fixtures + embed**

Create the three JSON files with the same content as `n8n/whatsapp-bot/reference/example-requests/{zusage,absage,statistik}.json` (copy verbatim).

Create `web/bottest/embed.go`:

```go
package bottest

import "embed"

//go:embed examples/*.json
var Examples embed.FS
```

- [ ] **Step 3: Add nav entry**

In `web/templates/partials/nav.templ`, add a link to `/bot-test` labelled "Bot-Test" with `ActiveNav` key `bottest` (match the existing nav-item pattern in that file).

- [ ] **Step 4: Create the page template**

Create `web/templates/bottest/page.templ`:

```go
package bottest

type PageVM struct {
	DefaultKind string // "statistik"
	DefaultJSON string // pretty example for DefaultKind
}

templ Page(vm PageVM) {
	<div class="page-header enter">
		<div class="eyebrow">WhatsApp-Bot</div>
		<h1>Bot-Test</h1>
		<p class="meta">Beispiel-Request auswählen, optional anpassen, an den Bot senden.</p>
	</div>
	<div class="bot-test enter">
		<div class="kind-switch" role="tablist">
			@kindButton("statistik", "Statistik", vm.DefaultKind)
			@kindButton("absage", "Absage", vm.DefaultKind)
			@kindButton("zusage", "Zusage", vm.DefaultKind)
		</div>
		<form hx-post="/bot-test/run" hx-target="#bot-response" hx-swap="innerHTML">
			<textarea id="bot-json" name="payload" rows="18" spellcheck="false">{ vm.DefaultJSON }</textarea>
			<button type="submit" class="btn-primary">An Bot senden</button>
		</form>
		<div id="bot-response" class="bot-response"></div>
	</div>
}

templ kindButton(kind, label, active string) {
	<button
		type="button"
		class={ "kind-btn", templ.KV("active", kind == active) }
		hx-get={ "/bot-test/example/" + kind }
		hx-target="#bot-json"
		hx-swap="innerHTML"
	>{ label }</button>
}
```

- [ ] **Step 5: Add handlers + routes (page + example loader)**

In `Routes()` add:

```go
	mux.HandleFunc("GET /bot-test", s.handleBotTest)
	mux.HandleFunc("GET /bot-test/example/{kind}", s.handleBotTestExample)
```

Add to `internal/web/server.go` (import `"github.com/michael/zumba-admin-ui/web/bottest"`, `"encoding/json"`, `"bytes"`):

```go
var botExampleKinds = map[string]bool{"statistik": true, "absage": true, "zusage": true}

func (s *Server) loadExample(kind string) (string, bool) {
	if !botExampleKinds[kind] {
		return "", false
	}
	raw, err := bottest.Examples.ReadFile("examples/" + kind + ".json")
	if err != nil {
		return "", false
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, raw, "", "  "); err != nil {
		return string(raw), true
	}
	return pretty.String(), true
}

func (s *Server) handleBotTest(w http.ResponseWriter, r *http.Request) {
	def, _ := s.loadExample("statistik")
	s.render(w, r, s.meta("Bot-Test", "bottest"),
		bottest.Page(bottest.PageVM{DefaultKind: "statistik", DefaultJSON: def}))
}

func (s *Server) handleBotTestExample(w http.ResponseWriter, r *http.Request) {
	body, ok := s.loadExample(r.PathValue("kind"))
	if !ok {
		http.Error(w, "unbekannt", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(body))
}
```

- [ ] **Step 6: Build**

Run: `cd n8n/zumba-admin-ui && make gen && go build ./...`
Expected: clean build. Manually: `make run`, open `/bot-test`, switch tabs → textarea updates.

- [ ] **Step 7: Commit**

```bash
git add n8n/zumba-admin-ui/internal/config/ n8n/zumba-admin-ui/internal/web/ n8n/zumba-admin-ui/web/bottest/ n8n/zumba-admin-ui/web/templates/
git commit -m "feat(admin-ui): bot-test page with example switcher + BOT_URL config"
```

---

### Task 7 — C2: Bot-Test run proxy + response rendering

**Files:**
- Create: `web/templates/bottest/response.templ`
- Modify: `internal/web/server.go` (route + handler + small bot client)
- Test: `internal/web/bottest_test.go` (new)

**Interfaces:**
- Consumes: `cfg.BotURL`, `bottest.Page`.
- Produces: `POST /bot-test/run`; `botOutcome` struct (matches bot `Outcome` JSON); `bottest.Response(vm bottest.ResponseVM)`; `bottest.ErrorPanel(msg string)`.

- [ ] **Step 1: Write failing test with a fake bot**

Create `internal/web/bottest_test.go`:

```go
package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestBotTestRunProxiesAndRenders(t *testing.T) {
	// Fake bot returns a statistik Outcome.
	bot := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/test" || r.Method != "POST" {
			t.Errorf("unexpected bot call: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"path":"statistik","message":"🍻 *ZUMBA STATS*\nfoo","recipient":"g@g.us"}`))
	}))
	defer bot.Close()

	cfg := testCfg()
	cfg.BotURL = bot.URL
	srv := New(newSpyStore(), cfg, false)

	form := url.Values{"payload": {`{"data":{"message":{"conversation":"statistik"}}}`}}
	req := httptest.NewRequest("POST", "/bot-test/run", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ZUMBA STATS") {
		t.Errorf("response did not render bot message:\n%s", rec.Body.String())
	}
}

func TestBotTestRunHandlesBotError(t *testing.T) {
	cfg := testCfg()
	cfg.BotURL = "http://127.0.0.1:0" // unreachable
	srv := New(newSpyStore(), cfg, false)
	form := url.Values{"payload": {`{}`}}
	req := httptest.NewRequest("POST", "/bot-test/run", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d (error path should still render 200 panel)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Fehler") {
		t.Errorf("expected error panel, got:\n%s", rec.Body.String())
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `cd n8n/zumba-admin-ui && make gen && go test ./internal/web/ -run BotTestRun -v`
Expected: FAIL — route 404.

- [ ] **Step 3: Create the response template**

Create `web/templates/bottest/response.templ`:

```go
package bottest

type ResponseVM struct {
	Path           string
	Classification string
	Action         string
	Message        string
	Recipient      string
	Date           string
	UserID         string
}

templ Response(vm ResponseVM) {
	<div class="bot-result">
		if vm.Path == "statistik" {
			<div class="wa-bubble">
				<pre>{ vm.Message }</pre>
			</div>
			<div class="wa-meta">Empfänger: { vm.Recipient }</div>
		} else if vm.Path == "classify" {
			<div class="badges">
				<span class={ "badge", "cls-" + vm.Classification }>{ vm.Classification }</span>
				<span class="badge action">{ vm.Action }</span>
			</div>
			<div class="wa-meta">User: { vm.UserID } · Datum: { vm.Date }</div>
			<div class="wa-bubble"><pre>{ vm.Message }</pre></div>
		} else {
			<div class="wa-meta">Ignoriert (Guards aktiv).</div>
		}
	</div>
}

templ ErrorPanel(msg string) {
	<div class="bot-error">
		<strong>Fehler:</strong> { msg }
	</div>
}
```

- [ ] **Step 4: Add route + proxy handler**

In `Routes()` add:

```go
	mux.HandleFunc("POST /bot-test/run", s.handleBotTestRun)
```

Add to `internal/web/server.go` (import `"io"`, `"net/http"`, `"time"` already present, `"strings"`):

```go
// botOutcome mirrors the whatsapp-bot Outcome JSON.
type botOutcome struct {
	Path           string `json:"path"`
	Classification string `json:"classification"`
	Action         string `json:"action"`
	Message        string `json:"message"`
	Recipient      string `json:"recipient"`
	Date           string `json:"date"`
	UserID         string `json:"userId"`
	Reason         string `json:"reason"`
}

func (s *Server) handleBotTestRun(w http.ResponseWriter, r *http.Request) {
	payload := r.FormValue("payload")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	client := &http.Client{Timeout: 35 * time.Second}
	req, err := http.NewRequestWithContext(r.Context(), "POST", strings.TrimRight(s.cfg.BotURL, "/")+"/test", strings.NewReader(payload))
	if err != nil {
		_ = bottest.ErrorPanel(err.Error()).Render(r.Context(), w)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		_ = bottest.ErrorPanel("Bot nicht erreichbar: " + err.Error()).Render(r.Context(), w)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		_ = bottest.ErrorPanel("Bot-Status " + resp.Status + ": " + string(body)).Render(r.Context(), w)
		return
	}
	var out botOutcome
	if err := json.Unmarshal(body, &out); err != nil {
		_ = bottest.ErrorPanel("Antwort nicht lesbar: " + err.Error()).Render(r.Context(), w)
		return
	}
	_ = bottest.Response(bottest.ResponseVM{
		Path: out.Path, Classification: out.Classification, Action: out.Action,
		Message: out.Message, Recipient: out.Recipient, Date: out.Date, UserID: out.UserID,
	}).Render(r.Context(), w)
}
```

- [ ] **Step 5: Run tests**

Run: `cd n8n/zumba-admin-ui && make gen && gofmt -w ./internal/web/ && go test ./internal/web/ -run BotTestRun -v`
Expected: PASS.

- [ ] **Step 6: Add minimal CSS for new components**

In `assets/static/css/styles.css` append styles for: `.toast`, `.toast-success`, `.toast-error`, `.toast-out`, `.toggle.is-absent`, `.toggle.is-present`, `.excluded-form`, `.btn-primary`, `.btn-danger`, `.bot-test`, `.kind-switch`, `.kind-btn.active`, `.wa-bubble pre`, `.badges .badge`, `.bot-error`. Match existing CSS-variable theming (light/dark). Keep mobile-first. (Use the `frontend-design` skill for this step.)

- [ ] **Step 7: Full build + test + manual check**

Run: `cd n8n/zumba-admin-ui && make gen && go vet ./... && go test ./... && go build ./...`
Expected: all PASS. Manual: run bot (`OUTPUT_MODE=stdout`) + admin-ui, open `/bot-test`, send each kind, confirm response renders and (for absage/zusage) the bot writes to DB.

- [ ] **Step 8: Commit**

```bash
git add n8n/zumba-admin-ui/internal/web/ n8n/zumba-admin-ui/web/templates/bottest/ n8n/zumba-admin-ui/assets/static/css/styles.css
git commit -m "feat(admin-ui): bot-test run proxy with response + error rendering"
```

---

## Part D — Deployment files (Helm, no cluster apply)

### Task 8 — D1: admin-ui Helm templates + values + helpers

**Files:**
- Create: `n8n/deployment/helm-charts/zumba/templates/admin-ui/{deployment,service,configmap,ingress-route}.yaml`
- Modify: `n8n/deployment/helm-charts/zumba/templates/_helpers.tpl`, `n8n/deployment/helm-charts/zumba/values.yaml`
- Modify: `n8n/zumba-admin-ui/.env.example` (add `BOT_URL`), `n8n/zumba-admin-ui/README.md` (note BOT_URL + bot-test), `n8n/CLAUDE.md` (mention Phase 2 done + bot-test)

**Interfaces:**
- Consumes: existing `_helpers.tpl` chart helpers, `whatsappBot`/`postgres` value keys.

- [ ] **Step 1: Add label helpers**

In `_helpers.tpl` append (mirroring the `whatsappBot` helpers added earlier):

```
{{/*
admin-ui specific labels
*/}}
{{- define "zumba.adminUi.labels" -}}
{{ include "zumba.labels" . }}
app.kubernetes.io/component: admin-ui
{{- end }}

{{/*
admin-ui selector labels
*/}}
{{- define "zumba.adminUi.selectorLabels" -}}
{{ include "zumba.selectorLabels" . }}
app.kubernetes.io/component: admin-ui
{{- end }}
```

- [ ] **Step 2: Add `adminUi` values block**

In `values.yaml` append:

```yaml
# admin-ui: Go-Service zur Verwaltung der Stammtisch-Daten + Bot-Test-Seite
adminUi:
  enabled: false
  image:
    repository: ghcr.io/undermyspell/zumba-admin-ui  # ggf. an Registry anpassen
    tag: "latest"
    pullPolicy: IfNotPresent
  resources:
    requests:
      cpu: 50m
      memory: 64Mi
    limits:
      cpu: 250m
      memory: 128Mi
  env:
    DB_NAME: zumba
    DB_PORT: "5432"
    DB_USER: n8n
    DB_SSLMODE: disable
    EVAL_PERIOD_START: "2025-12-01"
    EVAL_PERIOD_END: "2026-11-30"
    TZ: Europe/Berlin
  service:
    type: ClusterIP
    port: 8080
  ingress:
    enabled: true
    host: zumba-admin.example.com  # per-environment überschreiben
```

- [ ] **Step 3: Create configmap.yaml**

`templates/admin-ui/configmap.yaml`:

```yaml
{{- if .Values.adminUi.enabled -}}
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ include "zumba.fullname" . }}-admin-ui
  namespace: {{ .Values.namespace }}
  labels:
    {{- include "zumba.adminUi.labels" . | nindent 4 }}
data:
  PORT: {{ .Values.adminUi.service.port | quote }}
  DB_HOST: {{ include "zumba.fullname" . }}-postgres
  DB_PORT: {{ .Values.adminUi.env.DB_PORT | quote }}
  DB_NAME: {{ .Values.adminUi.env.DB_NAME | quote }}
  DB_USER: {{ .Values.adminUi.env.DB_USER | quote }}
  DB_SSLMODE: {{ .Values.adminUi.env.DB_SSLMODE | quote }}
  EVAL_PERIOD_START: {{ .Values.adminUi.env.EVAL_PERIOD_START | quote }}
  EVAL_PERIOD_END: {{ .Values.adminUi.env.EVAL_PERIOD_END | quote }}
  TZ: {{ .Values.adminUi.env.TZ | quote }}
  BOT_URL: http://{{ include "zumba.fullname" . }}-whatsapp-bot:{{ .Values.whatsappBot.service.port }}
{{- end }}
```

- [ ] **Step 4: Create service.yaml**

`templates/admin-ui/service.yaml`:

```yaml
{{- if .Values.adminUi.enabled -}}
apiVersion: v1
kind: Service
metadata:
  name: {{ include "zumba.fullname" . }}-admin-ui
  namespace: {{ .Values.namespace }}
  labels:
    {{- include "zumba.adminUi.labels" . | nindent 4 }}
spec:
  type: {{ .Values.adminUi.service.type }}
  ports:
    - port: {{ .Values.adminUi.service.port }}
      targetPort: 8080
      protocol: TCP
      name: http
  selector:
    {{- include "zumba.adminUi.selectorLabels" . | nindent 4 }}
{{- end }}
```

- [ ] **Step 5: Create deployment.yaml**

`templates/admin-ui/deployment.yaml`:

```yaml
{{- if .Values.adminUi.enabled -}}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "zumba.fullname" . }}-admin-ui
  namespace: {{ .Values.namespace }}
  labels:
    {{- include "zumba.adminUi.labels" . | nindent 4 }}
spec:
  replicas: 1
  revisionHistoryLimit: 2
  selector:
    matchLabels:
      {{- include "zumba.adminUi.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      labels:
        {{- include "zumba.adminUi.selectorLabels" . | nindent 8 }}
    spec:
      initContainers:
      - name: wait-for-postgres
        image: busybox:1.35
        command:
        - sh
        - -c
        - |
          until nc -z {{ include "zumba.fullname" . }}-postgres {{ .Values.postgres.service.port }}; do
            echo "Waiting for postgres..."
            sleep 2
          done
      containers:
      - name: admin-ui
        image: "{{ .Values.adminUi.image.repository }}:{{ .Values.adminUi.image.tag }}"
        imagePullPolicy: {{ .Values.adminUi.image.pullPolicy }}
        ports:
        - name: http
          containerPort: 8080
          protocol: TCP
        envFrom:
        - configMapRef:
            name: {{ include "zumba.fullname" . }}-admin-ui
        env:
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: postgres-secrets
              key: DB_POSTGRESDB_PASSWORD
        livenessProbe:
          httpGet:
            path: /healthz
            port: http
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /healthz
            port: http
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          {{- toYaml .Values.adminUi.resources | nindent 10 }}
{{- end }}
```

- [ ] **Step 6: Create ingress-route.yaml**

`templates/admin-ui/ingress-route.yaml`:

```yaml
{{- if and .Values.adminUi.enabled .Values.adminUi.ingress.enabled -}}
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: {{ include "zumba.fullname" . }}-admin-ui
  namespace: {{ .Values.namespace }}
  labels:
    {{- include "zumba.adminUi.labels" . | nindent 4 }}
spec:
  entryPoints:
    - web
  routes:
    - match: Host(`{{ .Values.adminUi.ingress.host }}`)
      kind: Rule
      services:
        - name: {{ include "zumba.fullname" . }}-admin-ui
          port: {{ .Values.adminUi.service.port }}
{{- end }}
```

- [ ] **Step 7: Render check**

Run:
```bash
cd n8n/deployment/helm-charts/zumba
helm template zumba . --set namespace=zumba-staging --set adminUi.enabled=true --set whatsappBot.enabled=true \
  | grep -A3 'admin-ui/'
```
Expected: all four admin-ui resources render; `BOT_URL: http://zumba-whatsapp-bot:8080` appears in the ConfigMap.

- [ ] **Step 8: Update env example + docs**

- Add `BOT_URL=http://localhost:8080` to `n8n/zumba-admin-ui/.env.example`.
- Note the Bot-Test page + `BOT_URL` in `n8n/zumba-admin-ui/README.md`.
- In `n8n/CLAUDE.md`, update the `zumba-admin-ui` description: Phase 2 (writes) + Bot-Test page now implemented; `adminUi` Helm templates added (per-env `host` + `enabled` override; needs an admin-ui image + `adminUi.image.repository`).

- [ ] **Step 9: Commit**

```bash
git add n8n/deployment/helm-charts/zumba/ n8n/zumba-admin-ui/.env.example n8n/zumba-admin-ui/README.md n8n/CLAUDE.md
git commit -m "feat(deployment): admin-ui Helm templates + docs (Phase 3, no apply)"
```

---

## Self-Review notes (coverage)

- Spec Teil A → Task A1 (Outcome, `/test`, bypass, guards-on webhook). ✓
- Spec Teil B → Task B1 (store writes), B2 (toast), B3 (excluded CRUD + Thursday validation), B4 (absence toggle on both detail pages). ✓
- Spec Teil C → Task C1 (config BOT_URL, embedded examples, page, example loader, nav), C2 (proxy run + render + error path). ✓
- Spec Teil D → Task D1 (Helm templates, helpers, values, env/docs, render check; no apply). ✓
- Error handling: non-Thursday → 422 + toast (B3); bot unreachable/non-200 → error panel (C2); DB errors → `s.fail` 500 + log. ✓
- Tests: bot `run`/`/test` (A1), mock writes (B1), excluded handlers (B3), toggle handlers (B4), bot-test proxy + error (C2). ✓
