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

> **Stand:** Abschnitte 1–7 beschreiben die erste Modellgeneration (v1, Juli 2026;
> deployt als classifier-service 0.1.2). Die Grundlagen gelten weiter. Daten,
> Aufteilung und Ergebnisse der zweiten Generation stehen in **Abschnitt 9**
> (Synthetics und Labeln) und **Abschnitt 10** (Modellvergleich v2).

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
uv run scripts/finetune_minilm.py  # MiniLM end-to-end finetunen → models/minilm_ft{,.joblib}
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

1. ~~`ml_messages`-Tabelle + persistentes Logging im `whatsapp-bot`~~ (läuft)
2. ~~Probebetrieb Modell vs. Gemini~~ (läuft, Labeln im Admin-UI – Abschnitt 9)
3. ~~Synthetics v1 von Hand labeln, neu trainieren, auf Whatsapp messen~~ (Abschnitt 10)
4. ~~Konfidenz-Schwelle kalibrieren~~ (je Modell auf den Synthetics, Abschnitt 10)
5. ~~Deployment: pure-Go-Inferenz~~ (`classifier-service/`)
6. Offene Schritte der Generation v2: Abschnitt 10, „Nächste Schritte"

---

## 9. Synthetics v1 und Labeln (Oktober 2026)

Ab der zweiten Modellgeneration gibt es drei Quellen. Benannt sind sie nach
ihrer Herkunft, nicht nach ihrer Rolle – die Rolle ändert sich mit der Zeit.

| Quelle | Tabelle | Herkunft | Rolle |
|---|---|---|---|
| **Synthetics** | `ml_synthetic_messages` | generiert, von Hand geprüft | Training |
| **Whatsapp** | `ml_messages` | echte Gruppennachrichten, vom Bot mitgeschrieben | Test (Hauptmetrik) |
| **Manuell** | `ml_test_messages` | selbst geschrieben auf `/ml-test` | Prüffälle, getrennt ausgewiesen |

- **Training nur mit Synthetics, Test nur mit Whatsapp.** Keine Synthetics im
  Test (das wäre Generator gegen Generator), kein 70/30 – das Verhältnis ergibt
  sich aus den zwei Töpfen. Hyperparameter per Cross-Validation auf den
  Synthetics.
- **Echte Nachrichten ins Training** erst, wenn genug da sind (Richtwert ~300
  echte, davon ≥ 30 Zusagen), dann rollierend: ältere ins Training, die
  jüngsten Wochen als Test.
- **Gelabelt wird alles im Admin-UI** (Menüpunkte Synthetics · Whatsapp ·
  Manuell, Seite `/ml-daten`). „Offen" heißt: in dieser Runde noch nicht von
  Hand gelabelt (`labeled_at` leer). Tasten 1/2/3, 0 verwirft eine Synthetic.

### Fachliche Regeln beim Generieren

- **Jede Nachricht ist eine Donnerstags-Nachricht über heute Abend.** Der Bot
  wertet nur donnerstags aus, immer für den laufenden Donnerstag. Nichts, was
  einen anderen Tag voraussetzt („gestern abgesagt").
- **Zukunfts-Ab- oder -Zusagen gibt es nicht** – „nächste Woch bin i raus" ist
  `invalid`. Eine heutige Absage darf die Zukunft nebenbei erwähnen.
- **Jede Zusage ist eine Rücknahme und klingt auch so** („doch", „wieder",
  „Absage zruck", weggefallener Grund). Anwesenheit ist Standard – ein bloßes
  „bin dabei" ohne Bezug schreibt niemand. In v1 wurden 46 solche Vorschläge
  nachträglich durch Rücknahmen ersetzt.
- **Grenzfälle bekommen kein Vorschlags-Label** (`generated_label` leer, `art`
  `grenzfall_<untertyp>`): später kommen, bedingt, mehrere Aussagen, andere
  Person, mehrdeutige Emojis, Ironie. Wie sie gelabelt werden, entscheidet der
  Mensch – danach lässt sich die Regel aus den Labels ablesen.
- **Keine echten Namen**, und die echten Nachrichten dienen nur als
  Stilvorlage – nie kopieren oder umformulieren, sie sind das Testset.

### Menge und Verteilung

Ziel: etwa 1.200 geprüfte Nachrichten. Generiert werden etwa 1.450, weil beim
Durchgehen erfahrungsgemäß 15–20 % rausfliegen. Mehr Menge bringt bei
TF-IDF + LogReg ab ~1.000 kaum etwas; Engpass sind echte Daten.

| Label | Arten (generiert) | Summe |
|---|---|---|
| `false` | echt 185 · kurz 70 · emoji 47 · lang 47 · haertefall 117 | 466 |
| `true` | echt 163 · kurz 60 · emoji 35 · lang 35 · haertefall 105 | 398 |
| `invalid` | orga 105 · smalltalk 105 · unsicher 82 · bloedsinn 70 · emoji 47 · haertefall 87 · haertefall_zukunft 30 | 526 |
| Grenzfall | spaeter 15 · bedingt 10 · mehrere 10 · andere_person 10 · emoji 8 · ironie 7 | 60 |

- **`invalid` am meisten**: inhaltlich die breiteste Klasse. **`true` am
  wenigsten**: wenige Arten, eine Zusage zu formulieren – mehr ergäbe nur
  Beinahe-Duplikate. Rest-Schieflage gleicht `class_weight="balanced"` aus.
- **Sprache je Zelle**: 35 % `bairisch_stark` (Oberpfälzer Einschlag), 40 %
  `bairisch_leicht`, 25 % `hochdeutsch`; reine Emojis und Links ohne Sprache.
- **Rauschen über alle Zellen**: etwa die Hälfte klein geschrieben, ein Drittel
  mit Tippfehlern oder ohne Umlaute; ❌ bei ~60 % der Absagen, vereinzelt auch
  in Zusagen und `invalid`, damit das Modell nicht „❌ = Absage" lernt.

**Stand v1 (14.10.2026):** 1.450 generiert von sieben parallelen Generatoren,
alle importiert (Dedup hat nichts verworfen: gegen echte Nachrichten höchstens
0,68 Ähnlichkeit, die ähnlichsten Paare untereinander – 0,8 bis 0,88 – sind fast
alle bewusst gesetzte Gegensatzpaare wie „Bin verhindert ❌" ↔ „Ich bin doch nicht
verhindert"). Sprache: 33 % stark, 37 % leicht, 24 % Hochdeutsch, 6 % ohne.

Was die Arten genau meinen, steht in der Generierungs-Vorgabe
[`docs/synthetics-vorgabe.md`](docs/synthetics-vorgabe.md).

### Ablauf

1. **Stilanker holen**: ~30 echte Nachrichten aus `ml_messages` (nur lesend).
   Sie kommen in die Vorgabe, aber nicht ins Repo.
2. **Generieren je Zelle**, in Teil-Batches von 10–15 Nachrichten mit jeweils
   eigener, konkreter Vorgabe (Grund × Sprache × Schreibweise). Mehrere
   Generatoren in getrennten Kontexten parallel – ein einzelner langer Lauf
   kollabiert auf dieselben 20 Schablonen. Ausgabe JSONL nach
   `data/generated/` (gitignored):
   `{"text", "generated_label", "art", "sprache", "batch_id"}`.
3. **Importieren** – erst Vorschau, dann schreiben:
   ```bash
   scripts/import_synthetic.sh data/generated/*.jsonl          # Vorschau + Bericht
   APPLY=1 scripts/import_synthetic.sh data/generated/*.jsonl  # in die Staging-DB
   ```
   Das Skript holt die echten Nachrichten und vorhandenen Synthetics lesend
   aus der DB und verwirft
   - exakte Duplikate (Schlüssel: normalisierter Text, **Emojis zählen mit** –
     sonst wären „Ich auch ❌" und „Ich auch 👍" gleich),
   - Fast-Duplikate echter Nachrichten (Kosinus über Zeichen-n-Gramme ≥ 0,9,
     Leakage-Schutz),
   - Fast-Duplikate untereinander (≥ 0,9).
   Ein zweiter Import derselben Dateien übernimmt nichts (`text_hash` ist
   eindeutig).
4. **Von Hand labeln** im Admin-UI; Unrealistisches verwerfen.
5. **Exportieren und trainieren**: nur gelabelte, nicht verworfene Synthetics.
   Die Nachrichten-Dateien liegen bewusst nicht in git – Quelle der Wahrheit
   ist die Tabelle nach dem Labeln.

---

## 10. Modellvergleich v2 (Oktober 2026)

Sieben Kandidaten, alle nur auf den 1.446 von Hand gelabelten Synthetics
trainiert und auf den echten Whatsapp-Nachrichten gemessen. **Ergebnis: Kein
neues Modell schlägt Gemini klar** – bester Kandidat NB-LogReg mit Macro-F1
0,92 auf den 87 eindeutigen Nachrichten (Gemini 0,94); die 95-%-Bereiche
überlappen vollständig, bei 11 Zusagen im Test entscheiden ein, zwei Fehler.
Größter Hebel sind die Daten (siehe unten), nicht das Modell.

### Pipeline

```bash
cd ml-classifier
uv sync --group embeddings                       # einmalig (Torch, Transformers)
scripts/export_data.sh                           # DB → data/v2/{synthetics,whatsapp,manuell}.jsonl (nur lesend, gitignored)
uv run --group embeddings scripts/train_v2.py    # sklearn-Kandidaten: Rastersuche per 5-fach-CV, OOF-Wahrscheinlichkeiten
uv run --group embeddings scripts/finetune_v2.py # MiniLM und gbert feintunen (GPU empfohlen), ebenfalls 5-fach
uv run --group embeddings scripts/evaluate_v2.py # Test auf Whatsapp/Manuell → models/v2/report.json
uv run --group embeddings scripts/crossval_real_v2.py  # Versuch: echte Nachrichten im Training
uv run --group embeddings scripts/train_final_v2.py    # Endmodelle: Synthetics + Whatsapp vor FINAL_START → models/v2/final/
```

Alle Artefakte liegen in `models/v2/` (gitignored): `<name>.joblib`, die
Out-of-fold-Wahrscheinlichkeiten `<name>.oof.npy`, `cv.json`, `report.json`,
`crossval_real.json`. Die Kandidaten selbst definiert `scripts/models_v2.py`.

### Aufteilung

| Topf | Quelle | Nachrichten (Zusage/Absage/invalid) | Rolle |
|---|---|---|---|
| Training | Synthetics, gelabelt, ohne verworfene | 1.446 (406/341/699) | Lernen; Hyperparameter per 5-facher CV |
| Test | Whatsapp, eindeutige Texte | 87 (11/32/44) | Hauptmetrik (Macro-F1 + Bootstrap-Bereich) |
| Test | Whatsapp, alle | 130 (12/74/44) | Fehler mit Wirkung wie im Betrieb |
| Test | Whatsapp, von 0.1.2 nie gesehen | 64 (10/25/29) | fairer Vergleich mit dem alten Modell |
| Prüffälle | Manuell | 10 (5/2/3) | Fall für Fall |
| Final-Set | Whatsapp ab 15.10.2026 | – | unangetastet bis zur Umstiegsentscheidung |

*Eindeutig* = gleicher Text bis auf Satzzeichen/Groß-Klein zählt einmal (sonst
dominiert 23-mal „Muss mich für heute abmelden ❌“). Die invalid-Schwelle (zu
unsicher → invalid) wird je Modell auf den Out-of-fold-Vorhersagen der
Synthetics gewählt, nie am Test.

Label-Regeln dieser Runde: Absage nur mit **ausdrücklichem Stammtisch-Bezug**
(abmelden, absagen, raus, ned dabei, ohne mi …; „heut ned“, „kann ned“ oder nur
ein Grund mit ❌ sind invalid – in der Gruppe wird nicht nur über den
Stammtisch geredet). Zusage = Rücknahme; „komm später“ = Zusage; Zukunft = invalid.

### Kandidaten und Ergebnisse

Macro-F1 auf den 87 eindeutigen Whatsapp-Nachrichten (mit Schwelle), dazu der
faire Vergleich auf den 64 von 0.1.2 nie gesehenen, Fehler mit Wirkung (auf den
87) und die Prüffälle:

| Modell | Macro-F1 (87) [95 %] | ungesehen (64) | fälschlich ab / Absage verp. / Zusage verp. / vertauscht | Prüffälle | CV | Go |
|---|---|---|---|---|---|---|
| Gemini (heute) | 0,94 [0,87–0,99] | 0,93 | 2 / 0 / 2 / 0 | – | – | API |
| 0.1.2 (heute, teils im Training gesehen) | 0,96 [0,88–1,00] | 0,95 | 2 / 0 / 2 / 2 | 8/10 | – | ja |
| **NB-LogReg** (Zeichen 2–4 + Wort 1–2, binär, NB-gewichtet) | **0,92** [0,83–0,98] | 0,92 | **1 / 0 / 1 / 0** | 9/10 | 0,88 | mit Erweiterung |
| MiniLM feingetunt | 0,91 [0,82–0,97] | **0,97** | 1 / 1 / 0 / 1 | **10/10** | 0,90 | nein (ONNX) |
| Zeichen 2–4 + Wort 1–2, TF-IDF + LogReg | 0,90 [0,80–0,97] | 0,90 | 2 / 0 / 2 / 1 | 7/10 | 0,87 | mit Erweiterung |
| SVM kalibriert (Zeichen 1–5) | 0,87 [0,77–0,95] | 0,87 | 4 / 0 / 3 / 3 | 7/10 | 0,85 | ja |
| gbert feingetunt | 0,87 [0,76–0,95] | 0,89 | 0 / 1 / 4 / 0 | 9/10 | 0,89 | nein (ONNX) |
| MiniLM eingefroren + LogReg | 0,83 [0,73–0,92] | 0,89 | 3 / 5 / 0 / 5 | 7/10 | 0,70 | nein |
| Zeichen 1–5 + LogReg (Bauart 0.1.2) | 0,82 [0,71–0,92] | 0,81 | 4 / 0 / 5 / 3 | 7/10 | 0,85 | ja |

Was daraus folgt:

- **„doch ⇒ Zusage“ ist eine gelernte Abkürzung.** Weil fast jede synthetische
  Zusage „doch“ enthält, lesen die Modelle „Sogi doch!“, „Hat er doch 😅“ oder
  „Eisprung is doch erst morgen“ als Zusage – 3 der 5 Fehler von NB-LogReg.
  Abhilfe: invalid-Synthetics, in denen „doch“ keine Anwesenheit meint.
- **Echte Nachrichten im Training helfen deutlich** (5-fach über Whatsapp,
  Duplikate je auf einer Seite): Zeichen+Wort 0,90 → **0,94** mit nur einem
  Fehler mit Wirkung, Zeichen+LogReg 0,82 → 0,92, MiniLM-ft 0,91 → 0,93,
  NB-LogReg bleibt bei 0,92. Das erklärt auch, warum 0.1.2 (mit echten Absagen
  trainiert) so stark ist.
- **Wort-n-Gramme lohnen sich**: die Zeichen-LogReg mit Wörtern dazu steigt von
  0,82 auf 0,90 – „doch“, „wieder“, „ned“ als ganze Wörter und Paare.
- **Feintuning schlägt eingefrorene Embeddings klar** (MiniLM 0,91 statt 0,83).
  gbert verpasst vier „komm später“-Zusagen und kennt keine Emojis (❌ wird
  zu `[UNK]`).
- **Report-Nachrichten des Bots** (Statistik über das Bot-Konto) landen im
  Test – der Bot sollte sie gar nicht erst klassifizieren.

### Runde 2: gezielte Synthetics und echte Nachrichten

150 zusätzliche Synthetics gegen die Fehler aus Runde 1 (60 invalid mit
„doch“/„wieder“, 30 invalid mit „ab“ als Zeitangabe, 40 lange invalid, 20
vorsorgliche Absagen mit „evtl. später“), danach 1.596 Synthetics. Die
Kandidaten stehen jetzt auf einem Plateau knapp unter Gemini:

| Modell | nur Synthetics R1 → R2 | + echte Nachrichten (R2, 5-fach) | Wirkung R2 (fälschl. ab / Absage verp. / Zusage verp. / vertauscht) | Prüffälle Endmodell |
|---|---|---|---|---|
| Gemini | 0,94 | – | 2 / 0 / 2 / 0 | – |
| **NB-LogReg** | 0,92 → **0,92** | 0,92 | 2 / 0 / 1 / 1 | **10/10** |
| MiniLM feingetunt | 0,91 → **0,93** | 0,92 | **0 / 2 / 1 / 0** | **10/10** |
| Zeichen + Wort | 0,90 → 0,79 | 0,92 | 6 / 0 / 6 / 5 | 8/10 |
| Zeichen + LogReg | 0,82 → 0,73 | 0,92 | 6 / 0 / 8 / 5 | 9/10 |

- Die vorsorglichen Absagen koppeln bei den TF-IDF-Modellen „später“ an
  „abmelden“ – Zeichen+Wort liest dann sogar „Meld mich wieder an“ als
  Absage. NB-LogReg und MiniLM bleiben robust.
- Die „doch“-Fehler bleiben, die Gegenbeispiele senken nur die Sicherheit.
- Mit echten Nachrichten im Training enden alle linearen Modelle bei 0,92 mit
  denselben fünf Fehlern – mehr lässt sich aus 87 Testnachrichten nicht lesen.
- **Entscheidung: NB-LogReg** – über beide Runden und mit wie ohne echte
  Nachrichten am stabilsten. Das Endmodell (`scripts/train_final_v2.py`,
  1.596 Synthetics + 130 Whatsapp vor dem 15.10.) geht in den Probebetrieb;
  gemessen wird auf dem Final-Set (`FINAL_START` in `models_v2.py`).

Ergebnisse der Runde 1 bleiben in `models/v2/*_runde1.json`.

### Nächste Schritte

1. ~~Gezielte Synthetics~~ (Runde 2, siehe oben).
2. ~~Echte Whatsapp-Nachrichten ins Training~~ (Endmodell); Test ist das
   Final-Set ab 15.10.2026 (~10 Nachrichten pro Woche).
3. NB-LogReg in den `classifier-service` portieren: binäre Zeichen-n-Gramme
   (2–4) + Wort-n-Gramme (1–2, Tokenizer `(?u)\b\w+\b|[❌✅]`), je Klasse
   Merkmal × NB-Gewicht × Koeffizient, Sigmoid, auf Summe 1 normiert; Parität
   wie bisher über `golden.json`. Dann classifier-service 0.2.0 in den
   Probebetrieb (ersetzt 0.1.2).
4. Report-Nachrichten des Bots vom Klassifizieren ausnehmen.
5. MiniLM nur, wenn eine ONNX-Laufzeit auf dem Pi (~470 MB) den kleinen
   Vorsprung rechtfertigt.
