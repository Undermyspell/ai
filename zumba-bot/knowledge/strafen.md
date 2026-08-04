# Strafen — Fachregeln

Das Strafen-Feature sanktioniert zwei Verhaltensweisen: dauerhaftes
Fernbleiben (automatisch) und unangekündigtes Nichterscheinen (manuell).
Die Kasse fließt in die Gruppenkasse — im Wrapped als „Strafenkasse"
ausgewertet (inkl. Maß-Umrechnung: 5 €/Maß).

## Strafarten

### Fehltage (automatisch)
Wer **5 Donnerstage in Folge** abgemeldet fehlt, zahlt **25 €**. Jeder
weitere Fehltag in der Serie kostet **+5 €** (6 Wochen = 30 €, 7 = 35 € …).

- Eine „Serie" = aufeinanderfolgende abgemeldete Donnerstage (Sperrtage
  unterbrechen nicht, sie zählen einfach nicht).
- Persistiert wird nur ein **Marker** (Person + erster Tag der Serie).
  Der Betrag wird **immer live berechnet** — korrigiert jemand nachträglich
  Anwesenheiten im Admin-UI, passt sich der Betrag an. Existiert die Serie
  nach einer Korrektur gar nicht mehr, verschwindet die Strafe aus den
  Reports von selbst.
- Erkennung und Marker-Anlage passieren beim echten Wochenreport-Lauf.

### No-Show (manuell)
Nicht abgemeldet und nicht gekommen: fester Betrag (Default **50 €**), wird
im Admin-UI angelegt. Der Betrag steht in der Strafe selbst.

## Der Reset-Mechanismus (kniffligster Teil)

**Begleichen oder Löschen einer Fehltage-Strafe setzt den Serienzähler
zurück.** Referenzfall aus der Anforderung: 6× gefehlt → 30 € → am Abend
beglichen → nächster Fehltag beginnt Serie 1 (nicht 35 €).

Präzise: Ein Reset-Zeitpunkt (Begleich-/Löschdatum) schneidet die Serie
zwischen zwei aufeinanderfolgenden Fehltagen `a` und `b`, wenn
`a ≤ Reset < b` (Tagesbasis). Der Fehltag `a` gehört noch zur alten Strafe,
`b` eröffnet die neue Serie. Deshalb dürfen gelöschte Strafen **nie** aus der
Tabelle entfernt werden — sie sind unsichtbar, aber als Reset-Marker aktiv.

## Sichtbarkeit im Wochenreport

| Status | Sichtbar? |
|---|---|
| offen | immer |
| beglichen | von der Begleichung bis **einschließlich des folgenden Donnerstags** (die Gruppe soll die Zahlung einmal sehen) |
| gelöscht | nie |

## Lebenszyklus

```
(auto erkannt, Serie ≥ 5)          (Admin-UI)
        Kandidat ──persist──▶ offen ──begleichen──▶ beglichen
                                │
                                └──löschen (soft)──▶ geloescht
```

Beglichene und gelöschte Strafen wirken beide als Reset; nur beglichene
tauchen (kurz) noch in Reports auf.

## Implementierungs-Konvention

Die komplette Fachlogik liegt wortgleich in **drei** Services
(`internal/penalty/` in whatsapp-bot, zumba-admin-ui, wrapped) — eigene
Go-Module, bewusste Duplikation. Jede Regeländerung muss in allen drei
Kopien landen. Die Tabellen-DDL wird von Bot **und** Admin-UI idempotent beim
Start angelegt (Deploy-Reihenfolge offen).
