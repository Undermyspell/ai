package web

import (
	"context"
	"testing"

	"github.com/michael/zumba-whatsapp-bot/internal/classifier"
)

func TestRunStatistikReturnsText(t *testing.T) {
	s, st, snd := newTestServer(classifier.Invalid, friday) // friday: guards would block, but statistik ignores guards
	out := s.run(context.Background(), groupMsg("statistik"), false, false)
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
	out := s.run(context.Background(), groupMsg("bin raus"), true, false)
	if out.Path != "classify" || out.Classification != "false" || out.Action != "marked_absent" {
		t.Fatalf("bad outcome: %+v", out)
	}
	if st.absentUserID != "user-123" {
		t.Errorf("MarkAbsent not called: %+v", st)
	}
}

func TestRunClassifyZusageMarksPresent(t *testing.T) {
	s, st, _ := newTestServer(classifier.Zusage, friday)
	out := s.run(context.Background(), groupMsg("bin dabei"), true, false)
	if out.Action != "marked_present" || st.presentUserID != "user-123" {
		t.Fatalf("bad outcome/store: %+v / %+v", out, st)
	}
}

func TestRunGuardsBlockWithoutBypass(t *testing.T) {
	s, st, _ := newTestServer(classifier.Absage, friday)
	out := s.run(context.Background(), groupMsg("bin raus"), false, false)
	if out.Path != "ignored" || out.Reason == "" {
		t.Fatalf("expected ignored with reason: %+v", out)
	}
	if st.absentUserID != "" {
		t.Error("no DB write expected when guards block")
	}
}
