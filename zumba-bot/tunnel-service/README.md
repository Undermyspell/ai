# tunnel-service

Schaltet die öffentlichen ngrok-Tunnel für **Wrapped** und **Admin-UI** an und aus.
Läuft als Sidecar neben dem ngrok-Agenten im selben Pod; das Admin-UI spricht nur
diesen Service an, nie den Agenten.

## Warum überhaupt ein eigener Service

Die Agent-API von ngrok kennt keine Authentifizierung und kann alles, was der Agent
kann — auch `{"proto":"tcp","addr":"zumba-postgres:5432"}`. Läge sie als ClusterIP
im Namespace, könnte jeder Pod die Datenbank ins Internet stellen. Deshalb:

- der Agent lauscht auf `127.0.0.1:4040`, erreichbar nur aus dem eigenen Pod,
- dieser Service kennt eine feste Ziel-Liste; die Adresse kommt **nie** aus dem Request,
- und er hält die Ablauffristen. Als Sidecar teilt er die Lebensdauer mit dem Agenten:
  stirbt der Pod, sind Tunnel *und* Fristen weg. Ein Timer im Admin-UI hätte jeden
  Deploy überlebt, der Tunnel wäre offen geblieben.

## API

| Aufruf | Zweck |
|---|---|
| `GET /status` | Zustand aller Ziele (inkl. `defaultTtlSeconds`, `maxTtlSeconds`) |
| `POST /open` | `{"target":"wrapped","ttlSeconds":7200}` — öffnet, `ttlSeconds` optional |
| `POST /close` | `{"target":"wrapped"}` |
| `POST /close-all` | Not-Aus (Knopf im Admin-UI, nächtlicher CronJob) |
| `GET /healthz` | Liveness/Readiness |

`open` und `close` kehren sofort zurück und liefern den neuen Status — der Auf- bzw.
Abbau läuft im Hintergrund. Genau deshalb gibt es die Zustände `opening` und
`closing`: das Admin-UI zeigt sie an, statt sekundenlang zu hängen.

Zustände: `inactive` · `opening` · `active` · `closing` · `error`

## Zähler

`Tunnel.Requests` kommt aus den Metriken des Agenten (`metrics.http.count`) und
bleibt bei uns **immer 0**: gezählt wird dort erst mit eingeschaltetem
Request-Inspektor, und der hält Anfragen und Antworten im Speicher des Agenten —
auf dem Pi nicht erwünscht. Das Feld bleibt im Status, wird aber nirgends
angezeigt.

## Abgleich mit dem Agenten

Alle 5 Sekunden: abgelaufene Tunnel schließen, dann mit `GET /api/tunnels` abgleichen.
Der Agent ist die Wahrheit.

- Tunnel weg, den wir für offen hielten → Zustand `inactive` (Agent wurde neu gestartet)
- Tunnel offen, den wir nicht kennen → **beim ersten Durchlauf** übernehmen (dieser
  Container kann neu gestartet sein, während der Agent weiterlief) mit frischer Frist;
  **danach** schließen
- Tunnel unter fremdem Namen → schließen, der Agent gehört uns allein

## Konfiguration

| Variable | Default | Zweck |
|---|---|---|
| `PORT` | `8080` | |
| `AGENT_API` | `http://127.0.0.1:4040` | lokale ngrok-Agent-API |
| `TUNNEL_TARGETS` | — | JSON-Liste `[{"name","label","addr"}]`, Pflicht |
| `DEFAULT_TTL` | `2h` | Laufzeit, wenn der Aufrufer keine nennt |
| `MAX_TTL` | `24h` | Obergrenze, deckelt jede Anfrage |
| `MOCK` | `false` | Attrappe statt echtem Agent (lokale Entwicklung) |

## Lokal

```bash
make dev     # MOCK=true, kein Authtoken nötig
make test
```
