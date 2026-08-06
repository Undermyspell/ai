package web

import (
	"context"
	"testing"
	"time"

	"github.com/michael/zumba-whatsapp-bot/internal/classifier"
	"github.com/michael/zumba-whatsapp-bot/internal/evolution"
	"github.com/michael/zumba-shared/penalty"
	"github.com/michael/zumba-whatsapp-bot/internal/store"
)

const testGroup = "000000000000-0000000000@g.us"

type fakeStore struct {
	statsCalled   bool
	absentUserID  string
	absentMessage string
	presentUserID string

	penaltyInput      penalty.Input // von PenaltyInputs geliefert
	autoStrafen       []string      // "userID|YYYY-MM-DD" der InsertAutoStrafe-Aufrufe
	penaltyInputCalls int
}

func (f *fakeStore) UserStats(context.Context, time.Time) ([]store.Stat, error) {
	f.statsCalled = true
	return []store.Stat{{Name: "A", Attendance: 1, Away: 0, Percent: 100}}, nil
}
func (f *fakeStore) MarkAbsent(_ context.Context, userID string, _ time.Time, msg string) error {
	f.absentUserID = userID
	f.absentMessage = msg
	return nil
}
func (f *fakeStore) MarkPresent(_ context.Context, userID string, _ time.Time) error {
	f.presentUserID = userID
	return nil
}
func (f *fakeStore) PenaltyInputs(context.Context, time.Time) (penalty.Input, error) {
	f.penaltyInputCalls++
	return f.penaltyInput, nil
}
func (f *fakeStore) InsertAutoStrafen(_ context.Context, marks []store.AutoStrafe) error {
	for _, m := range marks {
		f.autoStrafen = append(f.autoStrafen, m.UserID+"|"+m.Datum.Format("2006-01-02"))
	}
	return nil
}

type fakeClassifier struct{ result classifier.Result }

func (f fakeClassifier) Classify(context.Context, string) (classifier.Classification, error) {
	return classifier.Classification{Result: f.result, Raw: string(f.result), Model: "fake"}, nil
}

type fakeSender struct {
	number string
	text   string
	called bool

	imageNumber  string
	imageCaption string
	imagePNG     []byte
	imageErr     error
	imageCalled  bool
}

func (f *fakeSender) SendText(_ context.Context, number, text string) error {
	f.called = true
	f.number = number
	f.text = text
	return nil
}

func (f *fakeSender) SendImage(_ context.Context, number, caption string, png []byte) error {
	f.imageCalled = true
	f.imageNumber = number
	f.imageCaption = caption
	f.imagePNG = png
	return f.imageErr
}

func newTestServer(result classifier.Result, now time.Time) (*Server, *fakeStore, *fakeSender) {
	st := &fakeStore{}
	snd := &fakeSender{}
	loc, _ := time.LoadLocation("Europe/Berlin")
	s := New(st, fakeClassifier{result: result}, snd, testGroup, loc)
	s.Now = func() time.Time { return now }
	return s, st, snd
}

func groupMsg(text string) evolution.WebhookEvent {
	var ev evolution.WebhookEvent
	ev.Data.MessageType = "conversation"
	ev.Data.Key.RemoteJid = testGroup
	ev.Data.Key.ParticipantAlt = "user-123"
	ev.Data.PushName = "Tester"
	ev.Data.Message.Conversation = text
	return ev
}

var (
	thursday = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC) // 2026-01-01 ist ein Donnerstag
	friday   = time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
)

func TestStatistikSendsStats(t *testing.T) {
	s, st, snd := newTestServer(classifier.Invalid, thursday)
	s.run(context.Background(), groupMsg("Statistik"), false, false, s.today())
	if !st.statsCalled {
		t.Error("UserStats not called")
	}
	if !snd.called || snd.number != testGroup {
		t.Errorf("SendText not called correctly: %+v", snd)
	}
}

func TestAbsageMarksAbsent(t *testing.T) {
	s, st, _ := newTestServer(classifier.Absage, thursday)
	s.run(context.Background(), groupMsg("bin raus heute"), false, false, s.today())
	if st.absentUserID != "user-123" || st.absentMessage != "bin raus heute" {
		t.Errorf("MarkAbsent wrong: %+v", st)
	}
	if st.presentUserID != "" {
		t.Error("MarkPresent should not be called")
	}
}

func TestZusageMarksPresent(t *testing.T) {
	s, st, _ := newTestServer(classifier.Zusage, thursday)
	s.run(context.Background(), groupMsg("bin doch dabei"), false, false, s.today())
	if st.presentUserID != "user-123" {
		t.Errorf("MarkPresent wrong: %+v", st)
	}
	if st.absentUserID != "" {
		t.Error("MarkAbsent should not be called")
	}
}

func TestInvalidDoesNothing(t *testing.T) {
	s, st, _ := newTestServer(classifier.Invalid, thursday)
	s.run(context.Background(), groupMsg("schönes wetter heute"), false, false, s.today())
	if st.absentUserID != "" || st.presentUserID != "" {
		t.Errorf("no DB action expected: %+v", st)
	}
}

func TestNotThursdayDoesNothing(t *testing.T) {
	s, st, _ := newTestServer(classifier.Absage, friday)
	s.run(context.Background(), groupMsg("bin raus"), false, false, s.today())
	if st.absentUserID != "" {
		t.Error("no action expected on non-Thursday")
	}
}

func TestWrongGroupDoesNothing(t *testing.T) {
	s, st, _ := newTestServer(classifier.Absage, thursday)
	ev := groupMsg("bin raus")
	ev.Data.Key.RemoteJid = "someone-else@g.us"
	s.run(context.Background(), ev, false, false, s.today())
	if st.absentUserID != "" {
		t.Error("no action expected for other group")
	}
}

func TestNonConversationDoesNothing(t *testing.T) {
	s, st, _ := newTestServer(classifier.Absage, thursday)
	ev := groupMsg("bin raus")
	ev.Data.MessageType = "imageMessage"
	s.run(context.Background(), ev, false, false, s.today())
	if st.absentUserID != "" {
		t.Error("no action expected for non-conversation")
	}
}

func TestFromMeUsesSender(t *testing.T) {
	ev := groupMsg("bin raus")
	ev.Data.Key.FromMe = true
	ev.Sender = "owner-jid"
	s, st, _ := newTestServer(classifier.Absage, thursday)
	s.run(context.Background(), ev, false, false, s.today())
	if st.absentUserID != "owner-jid" {
		t.Errorf("fromMe should use sender, got %q", st.absentUserID)
	}
}
