package trace

import (
	"fmt"
	"strconv"

	"github.com/michael/zumba-admin-ui/internal/store"
)

// F formatiert eine Koordinate für SVG-Attribute (Template-Helfer).
func F(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }

// MarkerFor wählt die Pfeilspitze passend zum Kanten-Zustand.
func MarkerFor(state string) string {
	if state == "taken" {
		return "url(#arrow-taken)"
	}
	return "url(#arrow-skip)"
}

// Der Bot-Flow ist ein fester, kleiner Graph. Wir legen die Knoten einmal von
// Hand aus; der aufgezeichnete Trace eines Events steuert nur, welche Knoten/Kanten
// hervorgehoben (taken), ausgegraut (skipped), fehlgeschlagen (fail) oder
// fehlerhaft (error) gerendert werden.

const (
	viewW    = 1040.0
	viewH    = 916.0
	nodeW    = 210.0
	nodeH    = 72.0
	colLeft  = 175.0 // Karten-Mittelpunkt linke Spalte
	colMid   = 520.0
	colRight = 865.0
)

type nodeDef struct {
	id    string
	cx, y float64 // Karten-Mittelpunkt-x, Oberkante-y
	icon  string
	title string // Default-Titel (falls Knoten nicht erreicht wurde)
}

type edgeDef struct {
	id       string
	from, to string
	label    string
	x1, y1   float64 // Quelle: Unterkante-Mitte
	x2, y2   float64 // Ziel: Oberkante-Mitte
}

// Feste Topologie (top-down; alle Kanten verlaufen abwärts).
var nodes = []nodeDef{
	{store.NodeReceived, colMid, 20, "📨", "Webhook empfangen"},
	{store.NodeCheckStatistik, colMid, 140, "❓", `"statistik"?`},
	{store.NodeBuildStats, colLeft, 290, "📊", "Statistik berechnen"},
	{store.NodeGuardType, colMid, 290, "🛡️", "messageType?"},
	{store.NodeSendStats, colLeft, 430, "📤", "An Gruppe senden"},
	{store.NodeGuardGroup, colMid, 430, "🛡️", "Zumba-Gruppe?"},
	{store.NodeIgnored, colRight, 430, "🚫", "Ignoriert"},
	{store.NodeGuardThursday, colMid, 560, "🛡️", "Donnerstag?"},
	{store.NodeClassify, colMid, 690, "🤖", "Classifier"},
	{store.NodeMarkAbsent, colLeft, 820, "📝", "Absage: DB-Insert"},
	{store.NodeMarkPresent, colMid, 820, "✅", "Zusage: DB-Delete"},
	{store.NodeNoAction, colRight, 820, "➖", "keine Aktion"},
}

func anchor(cx, yTop float64) (botX, botY, topX, topY float64) {
	return cx, yTop + nodeH, cx, yTop
}

var edges = mkEdges()

func mkEdges() []edgeDef {
	pos := map[string]nodeDef{}
	for _, n := range nodes {
		pos[n.id] = n
	}
	def := func(from, to, label string) edgeDef {
		s, t := pos[from], pos[to]
		return edgeDef{id: from + "->" + to, from: from, to: to, label: label,
			x1: s.cx, y1: s.y + nodeH, x2: t.cx, y2: t.y}
	}
	return []edgeDef{
		def(store.NodeReceived, store.NodeCheckStatistik, ""),
		def(store.NodeCheckStatistik, store.NodeBuildStats, "ja"),
		def(store.NodeCheckStatistik, store.NodeGuardType, "nein"),
		def(store.NodeBuildStats, store.NodeSendStats, ""),
		def(store.NodeGuardType, store.NodeGuardGroup, "ja"),
		def(store.NodeGuardType, store.NodeIgnored, "nein"),
		def(store.NodeGuardGroup, store.NodeGuardThursday, ""),
		def(store.NodeGuardThursday, store.NodeClassify, ""),
		def(store.NodeClassify, store.NodeMarkAbsent, "false"),
		def(store.NodeClassify, store.NodeMarkPresent, "true"),
		def(store.NodeClassify, store.NodeNoAction, "invalid"),
	}
}

// NodeVM ist eine fertig gerenderte Knotenkarte.
type NodeVM struct {
	X, Y, W, H          float64
	Icon, Title, Detail string
	State               string // taken | skipped | fail | error
}

// EdgeVM ist eine fertig gerenderte Verbindung.
type EdgeVM struct {
	D              string
	LabelX, LabelY float64
	Label          string
	State          string // taken | skipped
}

// GraphVM bündelt alles, was das SVG-Template braucht.
type GraphVM struct {
	W, H  float64
	Nodes []NodeVM
	Edges []EdgeVM
}

func curve(x1, y1, x2, y2 float64) string {
	my := (y1 + y2) / 2
	return fmt.Sprintf("M %.1f %.1f C %.1f %.1f %.1f %.1f %.1f %.1f", x1, y1, x1, my, x2, my, x2, y2)
}

func nodeState(step store.TraceStep, reached bool) string {
	if !reached {
		return "skipped"
	}
	switch step.Outcome {
	case "error":
		return "error"
	case "fail":
		return "fail"
	default:
		return "taken"
	}
}

// BuildGraph mappt einen Trace auf die feste Topologie.
func BuildGraph(steps []store.TraceStep) GraphVM {
	byNode := make(map[string]store.TraceStep, len(steps))
	for _, s := range steps {
		byNode[s.Node] = s
	}

	g := GraphVM{W: viewW, H: viewH}
	for _, n := range nodes {
		step, reached := byNode[n.id]
		title := n.title
		detail := ""
		if reached {
			if step.Label != "" {
				title = step.Label
			}
			detail = step.Detail
		}
		g.Nodes = append(g.Nodes, NodeVM{
			X: n.cx - nodeW/2, Y: n.y, W: nodeW, H: nodeH,
			Icon: n.icon, Title: title, Detail: detail,
			State: nodeState(step, reached),
		})
	}
	for _, e := range edges {
		_, reached := byNode[e.to]
		state := "skipped"
		if reached {
			state = "taken"
		}
		g.Edges = append(g.Edges, EdgeVM{
			D:      curve(e.x1, e.y1, e.x2, e.y2),
			LabelX: (e.x1 + e.x2) / 2, LabelY: (e.y1 + e.y2) / 2,
			Label: e.label, State: state,
		})
	}
	return g
}
