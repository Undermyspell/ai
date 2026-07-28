// Package modeldata bettet die exportierten Modell-Artefakte ins Binary ein.
// Die Dateien werden von ml-classifier/scripts/export_weights.py erzeugt —
// hier nichts von Hand editieren.
//
// golden.json (echte Nachrichten im Klartext) wird bewusst NICHT eingebettet
// und ist nicht im Repo — der Golden-Test liest sie von Platte und skippt,
// wenn sie fehlt. Lokal erzeugen via export_weights.py.
package modeldata

import _ "embed"

//go:embed model.json.gz
var ModelGZ []byte
