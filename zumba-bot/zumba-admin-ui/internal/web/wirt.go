package web

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	sharedstore "github.com/michael/zumba-shared/store"
	"github.com/michael/zumba-shared/wirt"

	"github.com/michael/zumba-admin-ui/internal/store"
	"github.com/michael/zumba-admin-ui/internal/timeutil"
	wirtpage "github.com/michael/zumba-admin-ui/web/templates/wirt"
)

// wirt.go: Seite /tisch-erinnerung – die wöchentliche Bitte an den Wirt, den
// Tisch freizuhalten. Der Bot lässt den Text donnerstags um 10 Uhr von einem
// Sprachmodell schreiben und schickt ihn um 16 Uhr in die Gruppe. Dazwischen
// zeigt diese Seite ihn und kann ihn ablehnen – dann geht die feste
// Standardnachricht raus. Außerdem: ob überhaupt gesendet wird. Welches
// Modell schreibt, stellt die Seite /ki-modell ein (handleKIModellWirt). Der Bot liest alles vor jedem Lauf, ohne Neustart.

// wirtNow ist die Uhr der Seite (Tests stellen sie).
var wirtNow = time.Now

// wirtVerlauf: so viele Donnerstage zeigt der Verlauf.
const wirtVerlauf = 8

// wirtMarke steht in der Anzeige für die Markierung – die Nummer des Wirts
// kennt nur der Bot.
const wirtMarke = "Wirt"

// wirtBubble zerlegt eine fertige Nachricht an der Markierung, damit die
// Seite sie wie WhatsApp hervorheben kann.
func wirtBubble(text string) wirtpage.Bubble {
	vor, nach, ok := strings.Cut(wirt.Compose(text, wirtMarke), "@"+wirtMarke)
	b := wirtpage.Bubble{Vor: vor, Nach: nach}
	if ok {
		b.Marke = "@" + wirtMarke
	}
	return b
}

// modelLabel: Anzeigename eines Modells, sonst die ID.
func modelLabel(id string) string {
	if o, ok := findModel(id); ok {
		return o.Label
	}
	return id
}

// uhr: "10:02", an einem anderen Tag als ref mit Datum davor ("18.11. 10:02").
func uhr(t, ref time.Time) string {
	t = t.Local()
	if y, m, d := t.Date(); y != ref.Year() || m != ref.Month() || d != ref.Day() {
		return t.Format("02.01. 15:04")
	}
	return t.Format("15:04")
}

func (s *Server) wirtVM(ctx context.Context, now time.Time) (wirtpage.VM, error) {
	set, err := s.store.WirtSettings(ctx)
	if err != nil {
		return wirtpage.VM{}, err
	}
	termin := wirt.NextVersand(now)
	n, err := s.store.WirtNachricht(ctx, termin)
	if err != nil {
		return wirtpage.VM{}, err
	}
	verlauf, err := s.store.ListWirtNachrichten(ctx, wirtVerlauf+1)
	if err != nil {
		return wirtpage.VM{}, err
	}
	sperrtage, err := s.store.ListExcludedDays(ctx, timeutil.Period{Start: termin, End: termin})
	if err != nil {
		log.Printf("wirt: Sperrtage nicht lesbar: %v", err) // die Seite bleibt, nur ohne Hinweis
	}

	heute := timeutil.StartOfDay(now).Equal(termin)
	vm := wirtpage.VM{
		Termin:     timeutil.FormatDE(termin) + " · 16:00",
		TerminKurz: termin.Format("02.01."),
		Versand:    set.Versand,
		Standard:   wirtBubble(wirt.StandardText),
		ModelLabel: "noch keines gewählt",
	}
	if set.Model != "" {
		vm.ModelLabel = modelLabel(set.Model)
	}
	if !set.UpdatedAt.IsZero() {
		vm.Since = set.UpdatedAt.Local().Format("02.01.2006, 15:04")
	}

	// Zustand der nächsten Erinnerung.
	hatText := n != nil && n.Text != ""
	if hatText {
		b := wirtBubble(n.Text)
		vm.KI = &b
		vm.KIMeta = modelLabel(n.Model) + " · " + uhr(n.ErzeugtAm, now)
		if n.Quelle == sharedstore.WirtQuelleTest {
			vm.KIMeta += " · aus dem Bot-Test übernommen"
		}
	}
	wann := "am " + vm.TerminKurz
	if heute {
		wann = "heute"
	}
	switch {
	case !set.Versand:
		vm.Zustand = wirtpage.ZustandAus
		vm.Hinweis = "Der automatische Versand ist ausgeschaltet – am Donnerstag geht nichts an den Wirt, und es wird kein Text vorgeneriert."
	case len(sperrtage) > 0:
		vm.Zustand = wirtpage.ZustandSperrtag
		vm.Hinweis = timeutil.FormatDE(termin) + " ist ein Sperrtag – kein Stammtisch, keine Erinnerung."
	case hatText && n.AbgelehntAm != nil:
		vm.Zustand = wirtpage.ZustandAbgelehnt
		vm.Hinweis = "Du hast den KI-Text abgelehnt (" + uhr(*n.AbgelehntAm, now) + "). Um 16:00 geht " + wann + " die Standardnachricht raus."
	case hatText:
		vm.Zustand = wirtpage.ZustandKI
		vm.Hinweis = "Lehnst du nicht ab, geht der Text " + wann + " um 16:00 genau so in die Gruppe."
	case n != nil && n.Fehler != "":
		vm.Zustand = wirtpage.ZustandStandard
		vm.Hinweis = "Der KI-Text ist nicht entstanden (" + n.Fehler + "). Um 16:00 geht " + wann + " die Standardnachricht raus."
	case heute && now.Hour() >= wirt.GenerateHour:
		vm.Zustand = wirtpage.ZustandStandard
		vm.Hinweis = "Heute wurde kein KI-Text vorgeneriert. Um 16:00 geht die Standardnachricht raus."
	default:
		vm.Zustand = wirtpage.ZustandWartet
		vm.Hinweis = "Der KI-Text entsteht " + wann + " um 10:00. Bis dahin steht hier die Standardnachricht – sie geht raus, wenn kein Text entsteht oder du ihn ablehnst."
	}
	vm.Schritte = wirtSchritte(vm.Zustand, n, set, now)

	for _, v := range verlauf {
		if v.Tag.Format("2006-01-02") == termin.Format("2006-01-02") || len(vm.Verlauf) == wirtVerlauf {
			continue // der anstehende Termin steht oben
		}
		e := wirtpage.Eintrag{Datum: v.Tag.Format("02.01."), Text: strings.ReplaceAll(v.Text, wirt.Platzhalter, "@"+wirtMarke)}
		switch {
		case v.GesendetAls == sharedstore.WirtAlsKI:
			e.Label, e.Class = "KI-Text", "is-blue"
		case v.GesendetAls == sharedstore.WirtAlsStandard && v.AbgelehntAm != nil:
			e.Label, e.Class = "abgelehnt → Standard", "is-amber"
		case v.GesendetAls == sharedstore.WirtAlsStandard:
			e.Label, e.Class = "Standard", ""
			e.Text = "" // der KI-Text ging nicht raus – es gab keinen
		default:
			e.Label, e.Class = "nicht gesendet", "is-red"
		}
		vm.Verlauf = append(vm.Verlauf, e)
	}
	return vm, nil
}

// wirtSchritte ist der Ablauf des Donnerstags als Zeitleiste: schreiben,
// prüfen, senden.
func wirtSchritte(zustand string, n *store.WirtNachricht, set store.WirtSettings, now time.Time) []wirtpage.Schritt {
	schreibt := "schreibt " + modelLabel(set.Model)
	if set.Model == "" {
		schreibt = "Modell aus dem Deployment"
	}
	schreiben := wirtpage.Schritt{Label: "10:00 · KI schreibt den Text", Detail: schreibt, State: "wait"}
	pruefen := wirtpage.Schritt{Label: "bis 16:00 · du kannst ablehnen", State: "wait"}
	senden := wirtpage.Schritt{Label: "16:00 · Versand in die Gruppe", Detail: "Wirt wird markiert", State: "wait"}

	switch zustand {
	case wirtpage.ZustandAus, wirtpage.ZustandSperrtag:
		grund := "Versand ausgeschaltet"
		if zustand == wirtpage.ZustandSperrtag {
			grund = "Sperrtag"
		}
		for _, st := range []*wirtpage.Schritt{&schreiben, &pruefen, &senden} {
			st.State, st.Detail = "skipped", "entfällt – "+grund
		}
	case wirtpage.ZustandKI:
		schreiben.State, schreiben.Detail = "done", modelLabel(n.Model)+" · "+uhr(n.ErzeugtAm, now)
		pruefen.Detail = "nicht abgelehnt"
		senden.Detail = "KI-Text, Wirt wird markiert"
	case wirtpage.ZustandAbgelehnt:
		schreiben.State, schreiben.Detail = "done", modelLabel(n.Model)+" · "+uhr(n.ErzeugtAm, now)
		pruefen.State, pruefen.Detail = "done", "abgelehnt "+uhr(*n.AbgelehntAm, now)
		senden.Detail = "Standardnachricht, Wirt wird markiert"
	case wirtpage.ZustandStandard:
		schreiben.State, schreiben.Detail = "error", "kein Text entstanden"
		pruefen.State, pruefen.Detail = "skipped", "nichts zu prüfen"
		senden.Detail = "Standardnachricht, Wirt wird markiert"
	}
	return []wirtpage.Schritt{schreiben, pruefen, senden}
}

func (s *Server) handleWirt(w http.ResponseWriter, r *http.Request) {
	vm, err := s.wirtVM(r.Context(), wirtNow())
	if err != nil {
		s.fail(w, "wirt", err)
		return
	}
	s.render(w, r, s.meta("Tisch-Erinnerung", "wirt"), wirtpage.Page(vm))
}

func (s *Server) renderWirtRegion(w http.ResponseWriter, r *http.Request) {
	vm, err := s.wirtVM(r.Context(), wirtNow())
	if err != nil {
		s.fail(w, "wirt", err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := wirtpage.Region(vm).Render(r.Context(), w); err != nil {
		log.Printf("render wirt region: %v", err)
	}
}

// handleWirtVersand schaltet den automatischen Versand ein oder aus.
func (s *Server) handleWirtVersand(w http.ResponseWriter, r *http.Request) {
	var versand bool
	switch r.FormValue("versand") {
	case "an":
		versand = true
	case "aus":
	default:
		s.triggerToast(w, "error", "Unbekannte Auswahl.")
		http.Error(w, "unbekannt", http.StatusUnprocessableEntity)
		return
	}
	if err := s.store.SetWirtVersand(r.Context(), versand); err != nil {
		s.fail(w, "set wirt versand", err)
		return
	}
	log.Printf("🍽️  Tisch-Erinnerung: Versand → %v", versand)
	if versand {
		s.triggerToast(w, "success", "Versand ist an – donnerstags um 16:00 geht die Erinnerung raus.")
	} else {
		s.triggerToast(w, "success", "Versand ist aus – der Wirt bekommt nichts mehr, bis du ihn wieder einschaltest.")
	}
	s.renderWirtRegion(w, r)
}

// handleWirtAblehnen lehnt den KI-Text der nächsten Erinnerung ab bzw. nimmt
// das zurück (?zurueck=1). Der Tag kommt nie aus dem Formular: es geht immer
// um den nächsten Versand ab jetzt.
func (s *Server) handleWirtAblehnen(w http.ResponseWriter, r *http.Request) {
	ablehnen := r.FormValue("zurueck") == ""
	termin := wirt.NextVersand(wirtNow())
	ok, err := s.store.SetWirtAbgelehnt(r.Context(), termin, ablehnen)
	if err != nil {
		s.fail(w, "wirt ablehnen", err)
		return
	}
	switch {
	case !ok:
		s.triggerToast(w, "error", "Nichts geändert – es gibt keinen KI-Text oder die Erinnerung ist schon gesendet.")
	case ablehnen:
		log.Printf("🍽️  Tisch-Erinnerung %s: KI-Text abgelehnt", termin.Format("02.01."))
		s.triggerToast(w, "success", "Abgelehnt – am "+termin.Format("02.01.")+" geht die Standardnachricht raus.")
	default:
		log.Printf("🍽️  Tisch-Erinnerung %s: Ablehnung zurückgenommen", termin.Format("02.01."))
		s.triggerToast(w, "success", "Der KI-Text geht am "+termin.Format("02.01.")+" wieder raus.")
	}
	s.renderWirtRegion(w, r)
}
