# Vorgabe: synthetische WhatsApp-Nachrichten für den Zumba-Stammtisch-Klassifikator

Diese Vorgabe bekommt jeder Generator (Mensch oder LLM) – zusammen mit seiner
Zelle (Label × Art × Anzahl) und dem Pfad der Ausgabedatei. Konzept, Mengen
und Ablauf: [README, Abschnitt 9](../README.md#9-synthetics-v1-und-labeln-oktober-2026).

## Worum es geht

Eine Männerrunde aus der Oberpfalz (Raum Neumarkt) trifft sich jeden **Donnerstag** abends zum
Stammtisch („Zumba“). In der WhatsApp-Gruppe liest ein Bot mit. **Nur donnerstags** wertet er
Nachrichten aus, und zwar immer für **heute Abend**:

- `false` = Absage für heute Abend → der Bot trägt die Person als abwesend ein
- `true`  = Zusage bzw. Rücknahme einer heutigen Absage („meld mi wieder an“, „bin doch dabei“)
- `invalid` = alles andere → der Bot tut nichts

Anwesenheit ist Standard: Wer nichts schreibt, gilt als da. Deshalb sind Absagen der Normalfall, Zusagen
fast immer **Rücknahmen** („doch dabei“, „wieder an“).

Wir erzeugen Trainingsdaten für ein kleines Modell (Zeichen-n-Gramme + logistische Regression).
Ein Mensch prüft danach jede Nachricht von Hand. Das Label, das du vergibst, ist nur ein Vorschlag (`generated_label`).

## Harte Regeln

1. **Jede Nachricht ist eine Donnerstags-Nachricht über heute Abend.** Nichts, was einen anderen Tag
   voraussetzt („hab gestern abgesagt“ → falsch; „hab vorhin abgesagt“ → ok).
2. **Ab- oder Zusagen für die Zukunft gibt es nicht.** „Nächste Woch bin i raus“, „am 5.11. kann i ned“
   sind `invalid` (nur in der Zelle `haertefall_zukunft` erzeugen, sonst vermeiden).
   Eine Absage für heute darf aber nebenbei die Zukunft erwähnen („heut ned, nächste Woch wieder“ = `false`).
3. **Keine echten Namen aus der Gruppe** (vor dem Lauf aus der Tabelle `users` holen und dem Generator
   mitgeben – hier stehen sie bewusst nicht). Namen sparsam und erfunden (Schorsch, Hans, Maxl,
   Done, Wiggerl …), höchstens in jeder 20. Nachricht.
4. **Die Stilbeispiele unten nicht kopieren oder umformulieren.** Nur Ton, Länge, Emoji-Gewohnheiten und
   Dialekt übernehmen. Die echten Nachrichten sind das Testset – Paraphrasen davon wären ein Datenleck.
5. **„Später kommen“ gehört nur in die Grenzfall-Zelle.** In allen anderen Zellen keine Nachrichten der Art
   „komm später / wird später bei mir / komm nach“. Erlaubt ist das Muster der vorsorglichen Absage
   („meld mi vorsichtshalber ab, evtl schaff i's später no“ = `false`).
6. **Keine Beinahe-Duplikate.** Zwei Nachrichten, die sich nur in einem Emoji, einem Wort oder der
   Schreibweise unterscheiden, zählen als Duplikat – eine davon weglassen. Ein Filter wirft Ähnlichkeit
   ≥ 0,9 (Zeichen-n-Gramme) sowieso raus; was dort landet, ist verschwendet.

## Sprache (Feld `sprache`)

Je Zelle ungefähr: **35 % `bairisch_stark`**, **40 % `bairisch_leicht`**, **25 % `hochdeutsch`**.
Reine Emoji-Nachrichten und Links bekommen `sprache: null`.

- `bairisch_stark` (Oberpfälzer Einschlag): „I mou mi heid obmeldn“, „numal“, „a no“, „nert firti“, „Dou“,
  „kumm ned“, „hob“, „hod“, „woar“, „Leit“, „etz“, „fei“, „gell“, „mia“, „eich“, „vül“, „gscheid“, „glei“,
  „Woch“, „ganga“, „gwen“. Schreibung uneinheitlich wie echt (ned/net/nert, i/I, mi/Mi).
- `bairisch_leicht`: Umgangssprache mit Dialektspuren: „heut ned“, „meld mi ab“, „schaff's ned“, „bissl“, „a“ (auch).
- `hochdeutsch`: normales Deutsch, aber WhatsApp-Ton, nicht förmlich.

## Oberflächen-Rauschen (über alle Zellen verteilt)

- etwa die Hälfte ohne Großschreibung am Satzanfang oder komplett klein
- etwa ein Drittel mit Tippfehler, fehlenden Umlauten („fuer“, „schaffs“), Autokorrektur-Fehler oder ohne Satzzeichen
- Emojis wie in der Gruppe: bei Absagen oft ❌ (aber nicht immer, ~60 %), dazu gern eine **Emoji-Kette als Grund**
  (🇭🇷 🇮🇹 Urlaub · 🎂 Geburtstag · 🤒 🤧 krank · 🔨🧱🏗️ Baustelle · 🚜🌽 Feld · ⚽ · 👶 · 🏥 · ✈️ · 🚒 · 🎺 Musikverein).
  ❌ kommt vereinzelt auch in `true`/`invalid` vor („Termin ❌ also bin i dabei“) – sonst lernt das Modell „❌ = Absage“.

## Stilbeispiele (echte Nachrichten – NICHT kopieren)

Hier vor jedem Lauf etwa 30 echte Nachrichten aus `ml_messages` einfügen –
je Label einige, mit typischen Emoji-Ketten, Dialekt und Insider-Ton. Sie
stehen bewusst nicht im Repo (echte Gruppennachrichten, und sie sind das
Testset).

## Arten (Feld `art`)

`false`:
- `echt` – normaler Satz wie im Chat, mit oder ohne Grund (Arbeit/Schicht, krank, Kinder, Familie, Geburtstag,
  Urlaub/Reise, Baustelle/Haus, Hof/Feld, Verein, Fußball, Arzt, Feuerwehr, Musik …)
- `kurz` – 1 bis 4 Wörter („heut ned ❌“, „bin raus“)
- `emoji` – emoji-lastig, wenig bis keine Wörter, trotzdem eindeutig eine Absage („❌🤒🛌“, „heid ❌ 🇮🇹🏖️“)
- `lang` – 2 bis 4 Sätze mit Begründung, Grüßen, Nebeninfos
- `haertefall` – vorsorglich/sicherheitshalber ab (evtl. später doch); Antwort auf andere („i a ned ❌“,
  „bei mir a ned“); Verneinung („kann ned“, „doch ned“); mehrere Aussagen mit klarem Ende Absage;
  „hab vergessen mich abzumelden“ (für heute); „muss mich entschuldigen“ statt „abmelden“; Absage ohne
  Absage-Wort („Nullinger heid“, „mia ham zu“, „bin in Kroatien 🇭🇷“)

`true`:
- `echt` – Rücknahme oder Zusage für heute mit/ohne Grund („Termin is ausgfoin, bin doch dabei“)
- `kurz` – 1 bis 4 Wörter („doch dabei“, „bin do ✅“)
- `emoji` – emoji-lastig, aber mit eindeutiger Zusage („doch ✅🍻“)
- `lang` – 2 bis 4 Sätze
- `haertefall` – ❌ in einer Zusage („Termin ❌ also bin i dabei“); Verneinung dreht um („kann doch“,
  „doch ned raus“, „nix mit Absage“); Absage-Wörter in einer Zusage („vergesst mei Abmeldung“); „ich auch, bin dabei“

`invalid`:
- `orga` – Fragen und Infos zu Ort, Uhrzeit, Reservierung, Fahrgemeinschaft, Essen, Getränken, Kasse, Vereinsterminen
- `smalltalk` – Sprüche, Glückwünsche, Fußball, Wetter, Kommentare zu Fotos, Bier, Frotzeleien
- `unsicher` – „mal schauen“, „vielleicht“, „weiß no ned“, „geb no Bescheid“, „kommt drauf an“ – ohne Zu- oder Absage
- `bloedsinn` – Insider ohne Kontext, Tippfehler-Salat, Links, Sprachnachricht-Transkript-Fetzen, weitergeleitete Texte
- `emoji` – Emojis ohne Zu-/Absage-Bedeutung (😂, 👀, 🎯, 🤔, 🥹, 🙈 …), einzeln oder als Kette
- `haertefall` – Absage-Wörter in anderem Zusammenhang („im Fitnessstudio abgemeldet“); Fragen
  („wer hat sich heut alles abgemeldet?“); Vergangenheit („letzte Woch woar i ned do“); Kommentare zum Bot oder
  zur Statistik („da Bot hod mi falsch eitrogn“); Frotzeleien über die Absagen anderer
- `haertefall_zukunft` – Ab-/Zusagen nur für künftige Donnerstage („nächste Woch bin i raus“, „am 5.11. ned“,
  „ab nächster Woch wieder dabei“) – ohne Aussage über heute

Grenzfälle (`generated_label: null`, der Mensch entscheidet):
- `grenzfall_spaeter` – kommt später/verspätet, ohne Absage („wird halb neun bei mir“, „komm nach“)
- `grenzfall_bedingt` – „wenn …, dann bin i do“
- `grenzfall_mehrere` – widersprüchliche Aussagen in einer Nachricht
- `grenzfall_andere_person` – Ab-/Zusage für oder über andere, evtl. mit eigener Aussage
- `grenzfall_emoji` – mehrdeutige Einzel-Emojis mit Anwesenheits-Bezug (🍻, ✅, 👍, ❌ allein)
- `grenzfall_ironie` – ironische Ab-/Zusagen

## Arbeitsweise

- Arbeite in **Teil-Batches von 10–15 Nachrichten**, jeder mit einer eigenen, konkreten Vorgabe
  (z. B. „Grund Schicht/Arbeit, stark bairisch“, „Familie, mit Emoji-Kette“, „klein geschrieben, Tippfehler“).
  Nie „sei divers“ über alles – dann entstehen dieselben 20 Schablonen.
- Wechsle Satzbau, Länge, Wortwahl, Emoji-Position. Nicht jede Absage beginnt mit „Muss mich …“.
- Lies am Ende alles einmal durch und streiche Beinahe-Duplikate und Unrealistisches.

## Ausgabe

JSONL, eine Nachricht pro Zeile, **mit Python geschrieben** (`json.dumps(obj, ensure_ascii=False)`),
damit Anführungszeichen und Emojis sicher sind:

```json
{"text": "heid ned, Kind krank 🤒👶 ❌", "generated_label": "false", "art": "echt", "sprache": "bairisch_leicht", "batch_id": "v1-false-echt"}
```

- `batch_id` = `v1-<label>-<art>` (Grenzfälle: `v1-grenzfall-<untertyp>`, z. B. `v1-grenzfall-spaeter`)
- `generated_label`: `"true"`, `"false"`, `"invalid"` oder `null`
- Am Ende prüfen: jede Zeile gültiges JSON, Anzahl je Art stimmt, keine exakten Duplikate.
