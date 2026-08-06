# Deployment — fachliche Beschreibung

Das ganze System läuft selbst-gehostet auf einem **Raspberry Pi 5** (k3s,
Single-Node, aarch64) im Heimnetz. Verwaltet wird alles per **GitOps**: der
gewünschte Zustand steht im Git, ArgoCD gleicht den Cluster automatisch an.
Manuelle `kubectl apply`s sind tabu — Wahrheit ist das Repo.

## Umgebungen

Ein ArgoCD-ApplicationSet erzeugt pro Umgebung eine Application:

| Umgebung | Namespace | Status |
|---|---|---|
| Staging | `zumba-staging` | aktiv — hier läuft der produktive Betrieb |
| Production | `zumba-production` | vorgesehen, noch nicht ausgeprägt |

Jede Application hat zwei Quellen: das Helm-Chart (`helm-charts/zumba/`,
Werte aus `environments/<env>/values.yaml`) und Kustomize für die
SealedSecrets. Chart und Umgebungen müssen deshalb zusammen im
`deployment/`-Ordner bleiben.

## Was im Cluster läuft

| Komponente | Zweck |
|---|---|
| n8n | Workflow-Engine (Ursprung des Bots, weitere Automatisierungen) |
| Postgres | zentrale Datenbank (DBs `n8n` und `zumba`) |
| Evolution API | WhatsApp-Anbindung (Webhooks + Senden) |
| whatsapp-bot | der Bot (siehe whatsapp-bot.md) + Wochenreport-CronJob (Do 21:00) |
| zumba-admin-ui | Pflege-Oberfläche |
| zumba-classifier | ML-Schattenmodell für den Klassifikator-Vergleich |
| wrapped | Jahresrückblick (seit 08/2026) |
| zumba-renderer | HTML → PNG (headless Chromium) für die Statistik-Bild-Karte (seit 08/2026) |

Erreichbarkeit: nur im Heimnetz, HTTP über Traefik-IngressRoutes. n8n,
Admin-UI und Wrapped haben je einen eigenen Hostnamen — die konkreten Hosts
stehen pro Umgebung in `environments/<env>/values.yaml` (nicht in der Doku).

## Image-Versorgung (Besonderheit: keine Registry)

Die Go-Services werden **nativ auf dem Pi gebaut** und direkt in die
containerd-Registry von k3s importiert (`docker save | k3s ctr images
import`). Es gibt keinen Registry-Server; `pullPolicy: IfNotPresent` sorgt
dafür, dass der lokal importierte Stand verwendet wird.

Ablauf eines Deployments:
1. Quellcode zum Pi synchronisieren (rsync, ohne `.env`/`tmp`)
2. `docker build` auf dem Pi (arm64 nativ, kein QEMU) — Build-Kontext ist
   `zumba-bot/` (wegen des `shared/`-Moduls):
   `docker build -f <service>/Dockerfile -t <image> .`
   **Ausnahme renderer-service**: eigenständiges Modul ohne shared-Bezug,
   Build-Kontext ist `renderer-service/` selbst. Der Chromium-apt-Layer
   kann auf dem Pi transient fehlschlagen — Build einfach wiederholen.
3. Image in k3s importieren
4. Versions-Tag in `environments/staging/values.yaml` erhöhen,
   committen, pushen
5. ArgoCD synct (auto, ggf. mehrere Minuten — Refresh lässt sich anstoßen)

Wichtig: rsync kopiert den Working Tree — was deployt wird, sollte vorher
committet sein, sonst weicht Git vom Laufenden ab.

## Secrets

Secrets liegen **verschlüsselt im Git** (Bitnami SealedSecrets), pro
Umgebung mit eigenem Schlüssel. Entschlüsselte Secrets gehören nie ins Repo.
Das DB-Passwort konsumieren alle Services zur Laufzeit aus dem Secret
`postgres-secrets`. Neue Secrets entstehen über das Skript
`deployment/scripts/create-sealed-secret.sh`.

## Sonstiges

- Renovate hält Fremd-Images (n8n, Evolution, rclone …) aktuell und ändert
  dabei Chart-Values und lokales docker-compose zusammen.
- Postgres-Backup läuft als CronJob (Dump + rclone).
- Schema-Migrationen macht kein separates Tool: Bot und Admin-UI legen
  Tabellen/Spalten idempotent beim Start an (`strafen`,
  `stammtisch_abwesenheit.created_at`).
- Lokale Entwicklung: `make dev` im Repo-Root startet alles außer Postgres
  und Evolution API (die kommen aus dem Cluster, siehe `.env`-Dateien):
  Bot, Admin-UI, Wrapped, Classifier mit Hot-Reload auf dem Host, der
  Renderer als Docker-Container (Chromium + Emoji-Fonts identisch zum
  Cluster). `docker-compose.yml` enthält daneben noch n8n/Postgres/Evolution
  für Standalone-Betrieb.
