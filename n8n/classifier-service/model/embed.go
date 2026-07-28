// Package modeldata bettet die exportierten Modell-Artefakte ins Binary ein.
// Die Dateien werden von ml-classifier/scripts/export_weights.py erzeugt —
// hier nichts von Hand editieren.
package modeldata

import _ "embed"

//go:embed model.json.gz
var ModelGZ []byte

//go:embed golden.json
var GoldenJSON []byte
