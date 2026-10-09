# Disaster Recovery – Pi (k3s) neu aufsetzen

Was tun, wenn die SD-Karte oder der ganze Raspberry Pi kaputt ist: alles aus
Git, dem Sealed-Secrets-Schlüssel und dem Postgres-Backup auf einem neuen Pi
wiederherstellen.

> **Stand:** zusammengestellt am 2026-10-09 aus dem laufenden Setup – **noch
> nicht geprobt**, insbesondere der DB-Restore (Schritt 5). Wer es zum ersten
> Mal durchspielt: Abweichungen hier nachtragen.

## Was man vorher braucht

- Neuer Pi 5 mit Netzteil (5 V/5 A) und aktivem Kühler, SD-Karte oder besser
  NVMe-SSD, **LAN-Kabel**
- Zugriff auf das Git-Repo (`github.com/Undermyspell/ai`, öffentlich lesbar)
- **Passwortmanager:** `sealed-secrets-key.yaml` (privater Schlüssel des
  Sealed-Secrets-Controllers, siehe [README → Sealed-Secrets-Schlüssel](README.md#sealed-secrets-schlüssel-disaster-recovery))
- **Google Drive:** Ordner `zumba-backups/staging` (wöchentliche Dumps) – per
  lokalem `rclone` (Remote `gdrive`) oder im Browser
- **Bot-Handy** (WhatsApp-Nummer des Bots) zum Neukoppeln
- FritzBox-Zugang
- Arbeitsrechner mit `kubectl`, `kubeseal`, `docker buildx`, `rclone`

## Was wo liegt

| Baustein | Quelle | Bemerkung |
|---|---|---|
| Helm-Chart, Values, ArgoCD-Apps, Node-Drop-ins | Git | ArgoCD braucht keine Repo-Zugangsdaten |
| Code inkl. Classifier-Modell | Git | `classifier-service/model/model.json.gz` |
| Secrets (Postgres, Bot, Evolution, Admin-UI, ngrok, rclone) | Git (SealedSecrets) + Schlüssel aus dem Passwortmanager | alle mit `sealed-secrets-keymgkzw` versiegelt |
| Datenbanken `zumba`, `evolution` (+ Altlast `n8n`) samt Rollen | Drive-Backup (`pg_dumpall`, Fr 01:00, 90 Tage) | bis zu einer Woche alt |
| Container-Images | **keine Registry** – aus Git neu bauen | Tags stehen in `environments/staging/values.yaml` |
| WhatsApp-Kopplung | teils im Backup (Evolution-DB) | im Zweifel per QR neu koppeln |
| k3s-/ArgoCD-interne Secrets, ArgoCD-Admin-Passwort | – | werden bei der Installation neu erzeugt |

**n8n** wird nicht mehr gebraucht (seit 25.08.2026 abgeschaltet) und muss nicht
wiederhergestellt werden. Achtung, nur der Name lebt weiter: Der Postgres-User
aller Services heißt `n8n`, ebenso die DB, die Postgres beim Erststart anlegt.

## Ablauf

Die Reihenfolge zählt: **Schlüssel vor ArgoCD**, **DB-Restore vor den Images**.
Befehle laufen, wo nicht anders angegeben, auf dem Arbeitsrechner in
`zumba-bot/deployment/` des Repos.

### 1. Pi vorbereiten

1. Raspberry Pi OS Lite (64-bit) flashen: Hostname `raspberrypi5`, User `pi`,
   SSH mit Schlüssel.
2. **IP `192.168.178.46`** in der FritzBox fest an die **eth0-MAC des neuen
   Pi** vergeben (`cat /sys/class/net/eth0/address`). Alles hängt an dieser
   Adresse: kubeconfig, nip.io-Hostnamen, DB-Port 5433.
3. cgroups für k3s: an die eine Zeile in `/boot/firmware/cmdline.txt`
   ` cgroup_memory=1 cgroup_enable=memory` anhängen, neu starten.
4. Node-Drop-ins aus [`node/`](node/README.md) übernehmen (k3s wartet auf NTP –
   sonst verschieben sich nach dem Boot die CronJobs):
   ```bash
   sudo cp -r node/k3s.service.d node/systemd-time-wait-sync.service.d /etc/systemd/system/
   sudo systemctl daemon-reload
   ```
5. Docker installieren (für die nativen Image-Builds in Schritt 6):
   `curl -fsSL https://get.docker.com | sh`

### 2. k3s installieren

```bash
# Version wie im Upgrade-Plan (system-upgrade/k3s-server-plan.yaml)
curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=v1.37.1+k3s1 sh -
```

kubeconfig auf den Arbeitsrechner holen (`sudo cat /etc/rancher/k3s/k3s.yaml`),
`server:` auf `https://192.168.178.46:6443` setzen und als Kontext **`rpi5`**
in `~/.kube/config` mergen – Skripte und Doku erwarten diesen Namen.

### 3. Sealed-Secrets-Controller mit dem gesicherten Schlüssel

**Vor** ArgoCD, sonst kann der Controller die SealedSecrets aus Git nicht
entschlüsseln. Erst den Schlüssel, dann den Controller – und zwar über
[`sealed-secrets/`](sealed-secrets/kustomization.yaml), das setzt
`--key-renew-period=0` schon beim ersten Start. Ein nachträglicher Patch käme
zu spät: Der Controller sähe einen Schlüssel älter als 30 Tage und würde sofort
einen neuen erzeugen.

```bash
kubectl --context rpi5 apply -f sealed-secrets-key.yaml   # aus dem Passwortmanager
kubectl --context rpi5 apply -k sealed-secrets
kubectl --context rpi5 -n kube-system rollout status deploy/sealed-secrets-controller
# Prüfen: genau ein Schlüssel, und das Zertifikat gehört zu ihm
kubectl --context rpi5 get secret -n kube-system -l sealedsecrets.bitnami.com/sealed-secrets-key
kubeseal --context rpi5 --fetch-cert | openssl x509 -noout -enddate   # notAfter=Sep 21 … 2036
```

Danach die lokale Kopie wieder löschen (`shred -u sealed-secrets-key.yaml`).

### 4. ArgoCD und Apps

```bash
kubectl --context rpi5 create namespace argocd
kubectl --context rpi5 apply --server-side -k argocd/base
kubectl --context rpi5 -n argocd rollout status deploy/argocd-server
kubectl --context rpi5 apply -f argocd/argocd-app.yaml          # ArgoCD verwaltet sich selbst
kubectl --context rpi5 apply -f argocd/traefik-app.yaml
kubectl --context rpi5 apply -f argocd/system-upgrade-app.yaml
kubectl --context rpi5 apply -f argocd/applicationset.yaml      # → zumba-staging
```

`zumba-staging` synct jetzt. **Erwartet und in Ordnung:** Postgres läuft, die
zumba-Pods hängen in `ImagePullBackOff` (Images kommen erst in Schritt 6),
Evolution startet neu, weil seine DB noch fehlt. Genau deshalb legt bis zum
Restore niemand Tabellen an.

### 5. Datenbank zurückspielen

```bash
# Neuestes Backup holen
rclone lsl gdrive:zumba-backups/staging | sort -k2,3 | tail -3
rclone copy gdrive:zumba-backups/staging/zumba-pg-<DATUM>.sql.gz .

k() { kubectl --context rpi5 -n zumba-staging "$@"; }
k wait --for=condition=Ready pod/zumba-postgres-0 --timeout=300s
k exec zumba-postgres-0 -- psql -U n8n -d postgres -c '\l'
```

In der Liste sollten nur `n8n`, `postgres` und die Templates stehen. Hat
Evolution `evolution` schon angelegt, oder steht dort `zumba`: **nur auf dem
frisch aufgesetzten Cluster** vorher leeren – nie auf einem System mit Daten:

```bash
k exec zumba-postgres-0 -- psql -U n8n -d postgres \
  -c 'DROP DATABASE IF EXISTS evolution WITH (FORCE)' -c 'DROP DATABASE IF EXISTS zumba WITH (FORCE)'
```

Einspielen (dauert Sekunden):

```bash
gunzip -c zumba-pg-<DATUM>.sql.gz | k exec -i zumba-postgres-0 -- psql -U n8n -d postgres
```

Harmlos und erwartet: `role "n8n" already exists`, `database "n8n" already
exists`. Kurz prüfen und Evolution neu starten:

```bash
k exec zumba-postgres-0 -- psql -U n8n -d zumba \
  -c 'SELECT (SELECT count(*) FROM users) AS mitglieder, (SELECT count(*) FROM stammtisch_abwesenheit) AS absagen, (SELECT max(date) FROM stammtisch_abwesenheit) AS letzte'
k rollout restart deploy/zumba-evolution-api
```

### 6. Images bauen und importieren

Die Tags müssen **exakt** denen in `environments/staging/values.yaml`
entsprechen (`repository`/`tag` je Service), sonst findet k3s sie nicht.

**Bot und Admin-UI** – quer auf dem Arbeitsrechner, aus `zumba-bot/`:

```bash
cd ..   # zumba-bot/
docker buildx build --platform linux/arm64 -f whatsapp-bot/Dockerfile -t zumba-whatsapp-bot:<tag> --load .
docker buildx build --platform linux/arm64 -f zumba-admin-ui/Dockerfile -t zumba-admin-ui:<tag> --load .
for img in zumba-whatsapp-bot:<tag> zumba-admin-ui:<tag>; do
  docker save --platform linux/arm64 "$img" | ssh pi@192.168.178.46 'sudo k3s ctr images import -'
done
```

**Renderer, Classifier, Wrapped, Tunnel** – deren Dockerfiles brauchen ohne
QEMU einen nativen Build, also auf dem Pi:

```bash
ssh pi@192.168.178.46 'mkdir -p /tmp/zbuild'                                          # rsync legt das Ziel nicht an
rsync -az --exclude .git/ --exclude tmp/ ../ pi@192.168.178.46:/tmp/zbuild/          # zumba-bot/ hochladen
# auf dem Pi, in /tmp/zbuild:
docker build -f renderer-service/Dockerfile   -t zumba-renderer:<tag>   renderer-service/
docker build -f classifier-service/Dockerfile -t zumba-classifier:<tag> classifier-service/
docker build -f tunnel-service/Dockerfile     -t zumba-tunnel:<tag>     tunnel-service/
docker build -f wrapped/Dockerfile            -t zumba-wrapped:<tag>    .
for img in zumba-renderer:<tag> zumba-classifier:<tag> zumba-tunnel:<tag> zumba-wrapped:<tag>; do
  docker save "$img" | sudo k3s ctr images import -
done
```

Der Renderer-Build (Debian-`apt` für Chromium) scheitert gelegentlich
transient – einfach wiederholen. Die Pods starten von selbst, sobald ihr Image
da ist (sonst `k delete pod -l app.kubernetes.io/component=<name>`). Nicht
verwechseln: `docker` und k3s haben getrennte Image-Speicher, nur was per
`k3s ctr images import` drin ist, sieht der Cluster.

### 7. WhatsApp koppeln und Webhook prüfen

Evolution versucht, die Sitzung aus der wiederhergestellten DB weiterzunutzen.
Ist die Instanz `whatsapp` danach nicht verbunden:

1. Evolution-Manager öffnen (Evolution v2 liefert ihn unter `/manager` mit):
   `http://evolution-stage.192.168.178.46.nip.io/manager` – Server-URL ohne
   `/manager`, API-Key:
   `k get secret evolution-api-secrets -o jsonpath='{.data.AUTHENTICATION_API_KEY}' | base64 -d`
2. Instanz `whatsapp` → QR-Code anzeigen → auf dem **Bot-Handy**: WhatsApp →
   Verknüpfte Geräte → Gerät hinzufügen → scannen.

Dann muss der Webhook auf den Bot zeigen – ohne ihn läuft der Bot, bekommt aber
keine Nachrichten:

```bash
./switch-webhook.sh status    # zeigt das Ziel
./switch-webhook.sh bot       # falls es nicht der Bot ist
```

### 8. Prüfen

- `kubectl --context rpi5 get applications -n argocd` → alle Synced/Healthy;
  `k get pods` → alles Running.
- Admin-UI (`https://zumba-admin-stage.192.168.178.46.nip.io`) einloggen
  (Passwort: `ADMIN_PASSWORD` aus `admin-ui-secrets`), Dashboard zeigt die
  Mitglieder, Seite „Bild-Designs" die Warteschlange.
- Bot-Test: „Statistik" mit Ausgabe „Bild" als Dry-Run, dann einmal „📱 An mich".
- CronJobs vorhanden (`k get cronjobs`); Backup einmal von Hand anstoßen und
  in Drive nachsehen:
  ```bash
  k create job --from=cronjob/zumba-postgres-backup backup-nach-restore
  ```

## Wenn der Sealed-Secrets-Schlüssel fehlt

Dann sind die SealedSecrets in Git wertlos und jedes Secret muss neu entstehen
(`scripts/create-sealed-secret.sh`, siehe [README → Secrets Management](README.md#-secrets-management)):
Gemini-Key (Google AI Studio), ngrok-Token (ngrok-Dashboard), rclone-Config
(`rclone config` neu autorisieren), Evolution-API-Key und Admin-Passwort frei
wählen, Gruppen-/Vorschau-JID aus WhatsApp. **Postgres:** das neue Passwort
muss nach dem Restore zur Rolle `n8n` aus dem Dump passen –
`ALTER ROLE n8n PASSWORD '…'` im Postgres-Pod. Danach den neuen
Controller-Schlüssel sichern.

## Pflege

Dieses Dokument mitziehen, wenn ein Service, ein Secret oder ein Image dazukommt
oder sich Build-Wege ändern.
