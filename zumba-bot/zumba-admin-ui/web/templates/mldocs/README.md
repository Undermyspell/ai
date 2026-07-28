# ML-Klassifikator für Stammtisch-Nachrichten

Eigenes, lokal trainiertes Machine-Learning-Modell als Ersatz für den Gemini-Aufruf im
`whatsapp-bot`. Es klassifiziert WhatsApp-Nachrichten der Zumba-Gruppe in drei Klassen —
derselbe Kontrakt, den heute der LLM-Prompt (`system-prompt.txt`) erfüllt:

| Label | Bedeutung | Konsequenz im Bot |
|---|---|---|
| `true` | Zusage (kommt doch / wieder / später) | Absage-Eintrag für heute löschen |
| `false` | Absage | Eintrag in `stammtisch_abwesenheit` |
| `invalid` | weder noch (Smalltalk, "mal schauen", Orga, …) | nichts tun |

Diese Doku erklärt das Vorgehen so, dass man es auch ohne ML-Vorwissen nachvollziehen
kann. ML-Fachbegriffe werden beim ersten Auftreten erklärt.

---

## 1. Die Grundidee: Supervised Learning

Wir betreiben **Supervised Learning** ("überwachtes Lernen"): Wir zeigen dem Modell
viele Beispiel-Nachrichten, bei denen wir die richtige Antwort (das **Label**) schon
kennen — z.B. `("Muss mi heut abmelden ❌", false)`. Das Modell lernt daraus Muster
und kann anschließend auch Nachrichten einordnen, die es nie gesehen hat.

Das steht im Gegensatz zum bisherigen LLM-Ansatz: Gemini hat nichts aus unseren Daten
gelernt, sondern bekam bei jeder Nachricht einen Prompt mit Anweisungen und
entscheidet mit seinem Allgemeinwissen. Unser Modell dagegen ist winzig, kennt nur
diese eine Aufgabe — und genau deshalb reicht es: keine Cloud-Abhängigkeit, keine
Latenz, deterministisch, und später trivial in den Go-Service portierbar.

Ein ML-Modell kann nicht direkt mit Text rechnen. Der Weg ist immer:

```
Text  →  Zahlen-Vektor ("Features")  →  Klassifikator  →  Klasse + Wahrscheinlichkeit
```

Die zwei Entwurfsentscheidungen sind also: **Wie machen wir aus Text Zahlen?**
(Abschnitt 2) und **welcher Algorithmus lernt daraus?** (Abschnitt 3).

---

## 2. Vom Text zu Zahlen: TF-IDF über Zeichen-n-Gramme

### Was sind n-Gramme?

Ein **n-Gramm** ist ein Textausschnitt der Länge n. Wir arbeiten mit
**Zeichen**-n-Grammen der Längen 2 bis 4, d.h. wir schieben ein Fenster über die
Nachricht:

```
"abmelden"  →  ab, bm, me, el, ...        (2er)
               abm, bme, mel, eld, ...    (3er)
               abme, bmel, meld, ...      (4er)
```

Jede Nachricht wird zu einer Liste: "welche n-Gramme kommen darin vor, und wie oft?"

### Warum Zeichen statt ganzer Wörter?

Wegen Dialekt und Tippfehlern. Unsere echten Nachrichten enthalten `"abmelden"`,
`"abmelde"`, `"obmeldn"`, `"meld mi o"` — als *Wörter* betrachtet sind das vier
verschiedene, unbekannte Tokens. Als Zeichen-n-Gramme betrachtet **teilen sie viele
Teilstrings** (`me`, `eld`, `meld`, …). Das Modell erkennt die Verwandtschaft, ohne
dass irgendjemand Bairisch definiert haben muss. Genau dieser Effekt hat im
Modellvergleich (Abschnitt 6) den Ausschlag gegeben.

### Was macht TF-IDF daraus?

Nicht jedes n-Gramm ist gleich aussagekräftig. `en` kommt in fast jeder deutschen
Nachricht vor und verrät nichts; `meld` ist ein starkes Absage-Signal. **TF-IDF**
(Term Frequency – Inverse Document Frequency) gewichtet genau das:

- **TF**: Wie oft steht das n-Gramm in *dieser* Nachricht?
- **IDF**: In wie *wenigen* Nachrichten des gesamten Datensatzes kommt es vor?
  Seltene n-Gramme bekommen hohes Gewicht, Allerwelts-n-Gramme fast null.

Ergebnis pro Nachricht: ein **Vektor** (eine lange Zahlenreihe) mit einer Position je
bekanntem n-Gramm — zehntausende Positionen, fast alle 0, nur die n-Gramme der
Nachricht haben Werte. Man nennt das einen *dünn besetzten* (sparse) Vektor.

### Bewusste Grenze dieses Ansatzes

TF-IDF sieht nur Zeichen-Oberfläche, **kein Bedeutungsverständnis**. Eine völlig neue
Formulierung, die keine Teilstrings mit dem Training teilt, wird nicht erkannt. Für
unseren Fall ist das der richtige Trade-off: Es sind immer dieselben Sprechakte
("ich komme (nicht)") mit begrenztem Vokabular. Die Alternative mit
Bedeutungs-Vektoren haben wir trotzdem getestet — und sie verlor (Abschnitt 6).

---

## 3. Der Klassifikator: Logistische Regression

Auf den TF-IDF-Vektoren arbeitet eine **logistische Regression**. Trotz des Namens
ist das ein Klassifikations-, kein Regressionsverfahren, und eines der einfachsten
überhaupt:

- Das Modell lernt **pro Klasse ein Gewicht für jedes n-Gramm**. Anschaulich: `meld`
  bekommt ein stark positives Gewicht für `false`, `dabei` für `true`, `vielleich`
  für `invalid`.
- Für eine neue Nachricht werden die Gewichte der enthaltenen n-Gramme pro Klasse
  aufsummiert und in **Wahrscheinlichkeiten** umgerechnet, z.B.
  `{false: 0.91, invalid: 0.07, true: 0.02}`. Die höchste gewinnt.

"Lernen" heißt hier: Ein Optimierungsverfahren stellt die Gewichte so ein, dass die
Trainingsbeispiele möglichst richtig eingeordnet werden.

Drei Details unserer Konfiguration, und warum:

- **`class_weight="balanced"`**: Gleicht ungleiche Klassengrößen aus, damit das Modell
  nicht lernt "im Zweifel einfach `false` sagen, das ist eh am häufigsten".
- **Regularisierung (Parameter C=4)**: Eine eingebaute Bremse gegen **Overfitting** —
  das Auswendiglernen der Trainingsdaten statt des Lernens verallgemeinerbarer Muster.
  Kleines C = starke Bremse. Ein overfittetes Modell glänzt im Training und versagt
  an neuen Nachrichten.
- **Warum nicht die SVM?** Wir haben auch eine Support Vector Machine getestet (ein
  verwandtes Verfahren, das nur eine harte Ja/Nein-Trennlinie zieht). Sie war ähnlich
  gut, liefert aber **keine Wahrscheinlichkeiten**. Die brauchen wir für die geplante
  Bot-Logik: Ist sich das Modell zu unsicher (Top-Wahrscheinlichkeit unter einer
  Schwelle), geben wir `invalid` zurück — dieselbe "wenn unsicher → nichts tun"-Regel,
  die schon im Gemini-Prompt steht. Lieber nichts tun als falsch an-/abmelden.

---

## 4. Die Daten

### Grundprinzip: Synthetik fürs Training, echte Nachrichten für den Test

- **Training: ausschließlich synthetische (generierte) Nachrichten** — `data/synthetic.jsonl`
- **Test: ausschließlich echte Nachrichten aus der DB** — `data/real.jsonl` ("goldenes Testset")

Warum so strikt? Die eiserne Grundregel beim ML-Testen lautet: **Ein Modell darf nie
an Daten gemessen werden, mit denen es trainiert wurde** — sonst misst man
Auswendiglernen, nicht Können (wie eine Prüfung, deren Aufgaben man vorher hatte).
Unsere Trennung geht noch einen Schritt weiter und beantwortet die eigentlich
spannende Frage: *Transferiert das an künstlichen Beispielen Gelernte auf echte
Nachrichten?* Nur das zählt für den Produktiveinsatz.

### Echte Daten (`scripts/export_real.py`)

Zwei Quellen in der zumba-DB:

1. `stammtisch_abwesenheit.message` — jede Zeile ist per Definition eine Absage
   ⇒ Label `false`, sehr verlässlich.
2. `bot_trace` — dort protokolliert der Bot jede Klassifikation von Gemini
   (alle drei Labels). Achtung: 21-Tage-Retention, alte Einträge verschwinden.

Nach Deduplizierung (Text normalisiert: Kleinschreibung, Emojis/Satzzeichen raus):
**71 Nachrichten — 54 `false`, 16 `invalid`, 1 `true`**.

Das Ungleichgewicht ist kein Zufall, sondern liegt am Domänenmodell: Anwesenheit ist
der Default, man schreibt fast nur bei Absagen. Zusagen ("komme doch") sind selten —
und wegen der bot_trace-Retention gehen die wenigen auch noch verloren. Deshalb ist
geplant: eine Tabelle `ml_messages` ohne Retention, in die der Bot künftig jede
klassifizierte Nachricht dauerhaft schreibt (mit `verified`-Flag zum Handkorrigieren).

### Synthetische Daten — warum und wie generiert?

71 Beispiele sind zu wenig zum Trainieren, und für `true` gibt es fast nichts. Also
haben wir per LLM **~970 künstliche Nachrichten** erzeugt (Dateien
`data/synthetic_*.jsonl`), klassenbalanciert (~300 pro Klasse). Damit sie die echte
Vielfalt abdecken, wurde entlang expliziter **Variationsachsen** generiert, mit den
echten Nachrichten als Stil-Vorbild:

- **Dialekt-Mix**: ~40 % Hochdeutsch, ~30 % leicht bairisch ("heut", "ned", "a"),
  ~30 % deutlich bairisch ("I mou mi obmeldn", "Schau ma moi")
- **Begründungsvielfalt**: Arbeit, krank, Urlaub, Familie, Fußball, Wetter, …
- **Oberflächenrauschen**: Emojis, Kleinschreibung, Tippfehler, WhatsApp-Ton
- **Harte Fälle gezielt einstreuen** — die Fälle, an denen ein naives Modell scheitert:
  - "meld mich ab, komm evtl später no" ⇒ `false` (Absage trotz Später-Option!)
  - "bin doch dabei", "wird später bei mir" ⇒ `true` (spätes Kommen ist Zusage)
  - "mal schauen", "vielleicht", Orga für nächste Woche ⇒ `invalid`
- **Anti-Leakage**: In der Gruppe markiert ❌ Absagen. Stünde ❌ nur in
  `false`-Trainingsdaten, würde das Modell schlicht "❌ ⇒ Absage" lernen und sonst
  nichts (eine sogenannte Abkürzung / *shortcut*). Darum taucht ❌ vereinzelt auch in
  `true`/`invalid`-Beispielen auf ("Termin geplatzt ❌ also bin ich dabei 🍻").

`scripts/merge_synthetic.py` führt die Generator-Dateien zusammen, entfernt Duplikate
und **verwirft jede synthetische Nachricht, die (normalisiert) mit einer echten
übereinstimmt**. Sonst stünde ein Testfall de facto im Training — man nennt das
**Leakage** (Datenleck zwischen Training und Test), die häufigste Ursache für zu
schöne ML-Ergebnisse.

Die Dateien `synthetic_false_v2/v3.jsonl` sind gezielte Nachlieferungen: Nach der
ersten Evaluation fehlten dem Modell das "entschuldigen"-Vokabular, elliptische
Kurzformen ("ich a ❌") und die "vorsichtshalber ab"-Fälle — also wurden genau dafür
Beispiele nachgeneriert. So iteriert man datenzentriert: Fehler anschauen → Lücke im
Training identifizieren → Daten ergänzen → neu trainieren.

---

## 5. Training

```bash
cd ml-classifier
uv sync --group embeddings     # einmalig; ohne die Gruppe wird minilm übersprungen

uv run scripts/export_real.py      # DB → data/real.jsonl (DB_HOST etc. per Env)
uv run scripts/merge_synthetic.py  # synthetic_*.jsonl → data/synthetic.jsonl
uv run scripts/train.py            # Training + CV, Modelle → models/*.joblib
uv run scripts/evaluate.py         # Bewertung auf dem goldenen Testset
```

`train.py` trainiert alle drei Kandidaten und gibt zusätzlich für jeden einen
**Cross-Validation-Score** aus. **Cross-Validation (CV)** beantwortet die Frage "wie
stabil lernt das Modell?", ohne das Testset anzufassen: Die Trainingsdaten werden in
5 Teile geteilt; 5-mal wird auf 4 Teilen trainiert und am zurückgehaltenen 5. Teil
gemessen. Der Mittelwert (± Streuung) zeigt, ob das Modell verlässlich lernt oder ob
das Ergebnis vom zufälligen Datenschnitt abhängt.

Die **Hyperparameter** (Stellschrauben, die nicht gelernt, sondern von uns gesetzt
werden: n-Gramm-Bereich, Mindesthäufigkeit `min_df`, Regularisierung C) wurden per
Rastersuche (GridSearch) mit CV vorausgewählt.

---

## 6. Evaluation: Wie messen wir, ob das Modell gut ist?

`evaluate.py` bewertet jedes Modell in `models/` auf den 71 echten Nachrichten.

### Die Metriken, und warum genau diese

**Accuracy** (Anteil richtiger Antworten) allein wäre irreführend: Bei 54 von 71
`false`-Nachrichten erreicht ein Dummy, der stur "false" sagt, schon 76 % — und wäre
trotzdem nutzlos. Darum pro Klasse:

- **Precision** ("Wenn das Modell X sagt — wie oft stimmt das?"):
  Precision für `false` = Anteil echter Absagen unter allen als Absage
  klassifizierten Nachrichten. Niedrig ⇒ Leute werden fälschlich abgemeldet.
- **Recall** ("Wie viele echte X findet das Modell?"):
  Recall für `false` = Anteil erkannter Absagen an allen echten Absagen.
  Niedrig ⇒ Absagen rutschen durch.
- **F1** = Kombination aus beidem (harmonisches Mittel); nur gut, wenn beide gut sind.
- **Macro-F1** = Durchschnitt der drei F1-Werte, jede Klasse zählt gleich viel.
  Unsere Leitmetrik, weil sie sich von der `false`-Übermacht nicht blenden lässt —
  der Dummy von oben bekäme Macro-F1 ≈ 0.29.

### Die Confusion-Matrix

Eine Tabelle "wahre Klasse × vorhergesagte Klasse", die zeigt, *welche* Fehler
passieren — denn unsere Fehler sind unterschiedlich teuer:

- `true`↔`false`-Verwechslung = **teuerster Fehler**: jemand wird fälschlich
  ab- oder angemeldet, die Anwesenheitsstatistik wird falsch.
- `false→invalid` = billiger Fehler: der Bot tut nichts, der User merkt es und
  schreibt notfalls nochmal.

Zusätzlich druckt `evaluate.py` **jede Fehlklassifikation im Klartext**. Bei 71
Testfällen lernt man aus dem Lesen der Fehler mehr als aus jeder Kennzahl — genau
daraus entstanden die v2/v3-Nachlieferungen.

### Ergebnisse des Modellvergleichs

| Modell | Features | Acc | Macro-F1 | true↔false |
|---|---|---|---|---|
| **`tfidf_logreg`** ✅ | char-n-grams (2–4) + LogReg | **0.99** | **0.99** | **0** |
| `tfidf_svm` | gleiche Features + LinearSVC | 0.97 | 0.86 | 0 |
| `minilm_logreg` | Satz-Embeddings + LogReg | 0.89 | 0.57 | 3 |

Zum dritten Kandidaten: **Embeddings** sind Vektoren aus einem vortrainierten
neuronalen Netz (hier `paraphrase-multilingual-MiniLM-L12-v2`), bei denen
bedeutungsähnliche Sätze ähnliche Vektoren bekommen — "bin raus" und "ich komme
nicht" liegen nahe beieinander, obwohl sie kein Wort teilen. Klingt überlegen, verlor
hier aber deutlich: Das Netz wurde auf Standardsprache vortrainiert und kennt kein
Bairisch — "Sogi doch!" oder "obmeldn" landen irgendwo im Nirgendwo des Vektorraums.
Die "dummen" Zeichen-n-Gramme schlagen das große vortrainierte Netz, weil sie zur
Eigenart unserer Daten passen. Wichtige ML-Lektion: **das aufwendigere Modell ist
nicht automatisch das bessere.**

Einziger verbleibender Fehler des Siegers: `"Gruzefix.... Nullinger"` wird als
`invalid` statt `false` eingeordnet — eine Insider-Absage, die ohne Gesprächskontext
aus dem Text allein nicht erkennbar ist. Immerhin in der billigen Fehlerrichtung.

### Warum wir den 0.99 trotzdem nicht blind trauen

1. **Implizites Test-Set-Tuning**: Die finale Hyperparameter-Wahl und die
   v2/v3-Nachlieferungen orientierten sich an beobachteten Testfehlern. Damit ist das
   Testset in die Modellentwicklung eingeflossen und die 0.99 sind geschönt — nicht
   durch Betrug, sondern durch wiederholtes Hinschauen. Bei nur 71 Testfällen ließ
   sich das kaum vermeiden; man muss es aber wissen und benennen.
2. **Nur eine einzige echte Zusage** im Testset: Alle `true`-Metriken beruhen auf
   n=1 und sind statistisch wertlos, bis `ml_messages` echte Zusagen gesammelt hat.
3. **Konfidenz-Schwelle noch nicht kalibriert** — aus demselben Grund: Der
   Schwellen-Sweep in `evaluate.py` bricht scheinbar dramatisch ein, sobald die
   Schwelle die eine `true`-Nachricht überschreibt. Erst mit mehr echten Daten sinnvoll.

### Der eigentliche Abnahmetest: Shadow-Modus

Der ehrlichste Test kommt vor dem Cutover: Das Modell läuft im Bot **parallel zu
Gemini** — beide klassifizieren jede Nachricht, beide Ergebnisse werden geloggt, aber
nur Gemini entscheidet. Wöchentlich werden die Abweichungen (Disagreements) angesehen.
Erst wenn die Übereinstimmung über mehrere Wochen stabil hoch ist und die
`true`-Klasse echte Daten hat, übernimmt das Modell — mit kalibrierter
Konfidenz-Schwelle und `invalid` als Unsicherheits-Fallback.

---

## 7. Dateien

```
data/real.jsonl                  # goldenes Testset (DB-Export, nie im Training)
data/synthetic_*.jsonl           # Generator-Outputs (Rohteile, v2/v3 = Nachlieferungen)
data/synthetic.jsonl             # merged + dedupliziert = Trainingsdaten
scripts/export_real.py           # DB → real.jsonl
scripts/merge_synthetic.py       # Rohteile → synthetic.jsonl (Dedup, Leakage-Filter)
scripts/train.py                 # Training aller Kandidaten + Cross-Validation
scripts/evaluate.py              # Metriken, Confusion-Matrix, Sweep, Fehlerliste
scripts/common.py                # geteilte Bausteine (u.a. MiniLM-Encoder)
models/*.joblib                  # trainierte Modelle (nicht committen)
```

Datensatz-Format (JSONL, eine Nachricht pro Zeile):

```json
{"text": "Muss mi heut abmelden ❌", "label": "false", "source": "synthetic", "dialect": "leicht"}
```

## 8. Nächste Schritte

1. `ml_messages`-Tabelle + persistentes Logging im `whatsapp-bot` (füllt die
   `true`-Klasse, liefert Shadow-Modus-Daten)
2. Shadow-Modus Modell vs. Gemini, Disagreement-Review
3. Konfidenz-Schwelle kalibrieren, sobald genug echte Zusagen vorliegen
4. Deployment: Gewichte des `tfidf_logreg` als JSON exportieren → pure-Go-Inferenz
   (TF-IDF + Matrix-Vektor-Produkt, keine ML-Runtime nötig) in eigenem Service
