# 003 — Öffentlicher Zugang auf Zeit (ngrok)

Stand: 09/2026

## Ziel

Wrapped und das Admin-UI sollen sich gelegentlich ins Internet schalten lassen —
etwa um den Wrapped-Link in die Gruppe zu geben — und danach wieder zu sein.
Ohne Portfreigabe am Router, ohne dauerhafte Exposition, bedienbar aus dem
Admin-UI, mit sichtbarem Zustand (auf/ab/an/aus).

## Entscheidungen

**ngrok-Agent als Dauerläufer, Tunnel auf Knopfdruck.** Der Pod läuft immer
(`ngrok start --none`: verbunden, kein Tunnel). Geschaltet wird zur Laufzeit
über die Agent-API. Der Zustand liegt damit *nicht* im Git — ArgoCD hat nichts
zu synchronisieren und kämpft nicht gegen den Schalter.

Verworfen: **ngrok-Operator mit CRDs.** Ein vom UI angelegter CR wäre von
ArgoCDs Prune/Self-Heal wieder eingesammelt worden. Verworfen auch
**Deployment 0↔1 skalieren** — langsamer, und der Wunsch war ausdrücklich ein
dauerhaft laufender Pod.

**Eigener tunnel-service als Sidecar statt direkter Zugriff aus dem Admin-UI.**
Zwei Gründe:

1. *Sicherheit.* Die ngrok-Agent-API kennt keine Authentifizierung und kann
   alles, was der Agent kann — auch `{"proto":"tcp","addr":"zumba-postgres:5432"}`.
   Läge sie als ClusterIP im Namespace, könnte jeder Pod die Datenbank
   veröffentlichen. Jetzt lauscht sie auf `127.0.0.1:4040`, und davor steht ein
   Dienst, der genau zwei fest verdrahtete Ziele kennt.
2. *Lebensdauer.* Die Ablauffrist braucht einen Besitzer, der mit dem Tunnel
   stirbt. Ein Timer im Admin-UI hätte jeden Deploy überlebt — der Tunnel wäre
   offen geblieben. Als Sidecar teilen Agent und Aufpasser dasselbe Schicksal.

**Tunnel zeigt direkt auf den Service, nicht über Traefik.** Die IngressRoutes
matchen auf ihre Hostnamen (der ngrok-Host liefe ins Leere), und die
https-Umleitung des Admin-UI würde über ngrok zur Endlosschleife.

**Keine reservierte Domain.** Beide URLs sind zufällig; der Link wird beim
Teilen aus dem Admin-UI kopiert. Für eine Adminoberfläche ist eine wechselnde
Adresse ohnehin die bessere.

**Kein zusätzlicher Schutz vor dem Admin-Login** (bewusste Entscheidung des
Betreibers). Als Ausgleich: Bot-Test und ML-Test sind gesperrt, solange ein
Tunnel offen ist — der Preview-Modus verschickt echte WhatsApp-Nachrichten —
und der Login bremst Fehlversuche exponentiell (0,5 s → 30 s). Bewusst keine
Sperre nach N Versuchen: die ließe sich von außen auslösen, um den Besitzer
auszusperren.

## Zustandsmodell

```
inactive ──open──> opening ──ok──> active ──close/Ablauf──> closing ──> inactive
                      │                                        │
                      └──Fehler──> error <─────Fehler───────────┘
```

Auf- und Abbau laufen asynchron; genau dafür gibt es `opening`/`closing`, sonst
hinge die Seite ein paar Sekunden. Alle 5 s gleicht der tunnel-service mit dem
Agenten ab: der Agent ist die Wahrheit. Beim *ersten* Abgleich werden bereits
offene Tunnel übernommen (der Container kann neu gestartet sein, während der
Agent weiterlief) — danach gilt ein unbekannter Tunnel als Leiche und wird
geschlossen.

## Abschaltung, dreifach

1. Knopf im Admin-UI
2. Ablauffrist (Vorgabe 2 h, Auswahl 2/8/24 h, Deckel 24 h)
3. CronJob `zumba-ngrok-close-all` um 03:00 — stumpfes Netz für den Fall, dass
   Frist 2 nie greift

## Was dazukam

- `tunnel-service/` (neuer Go-Service, Attrappe per `MOCK=true`)
- `deployment/helm-charts/zumba/templates/ngrok/` (Deployment mit zwei
  Containern, ConfigMap, Service, CronJob), `ngrok.*` in den Values
- SealedSecret `ngrok-secrets` (`NGROK_AUTHTOKEN`) — muss vor `enabled: true`
  im Cluster liegen
- Admin-UI: Seite `/public`, Sperre für Bot-/ML-Test, Login-Bremse
