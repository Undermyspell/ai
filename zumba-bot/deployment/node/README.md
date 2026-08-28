# Node Configuration

Host-level systemd configuration for the k3s node (`raspberrypi5`, 192.168.178.46).

Everything else in `deployment/` is applied by ArgoCD. These files are **not** —
they live on the host filesystem below `/etc/systemd/system/` and have to be
copied to the Pi by hand. They are tracked here so the node config is versioned
and reviewable instead of only existing on the SD card.

## Directory Structure

```
node/
├── k3s.service.d/
│   └── wait-for-time.conf        # k3s waits for NTP before starting
├── systemd-time-wait-sync.service.d/
│   └── timeout.conf              # bounds the NTP wait so the boot can't hang
└── README.md
```

The layout mirrors the target paths: `node/<unit>.d/<file>` →
`/etc/systemd/system/<unit>.d/<file>`.

---

## Why: CronJobs silently stop firing after a reboot

The Pi 5 has an RTC but no battery, so the kernel comes up with the clock at
`1970-01-01` and systemd restores the last known timestamp until timesyncd gets
an answer from NTP. On 2026-08-27 that window looked like this:

```
kernel: rpi-rtc ... setting system clock to 1970-01-01T00:00:09 UTC
journal boot start:  2026-08-25 16:22:31   <- restored, ~38h in the past
k3s ExecMainStart:   2026-08-25 16:22:41   <- k3s starts against the wrong clock
timesyncd:           2026-08-27 06:58:23   Initial clock synchronization
```

The Kubernetes CronJob controller computes its requeue delay once
(`nextScheduleTimeDuration`) and hands it to `workqueue.AddAfter`. That timer is
**monotonic**, so the later forward jump of the wall clock does not shorten it —
every CronJob ends up delayed by exactly the size of the jump:

| CronJob | Schedule | Should have run | Actually queued for |
|---------|----------|-----------------|---------------------|
| `zumba-whatsapp-bot-weekly` | `0 21 * * 4` | Thu 2026-08-27 21:00 | Sat 2026-08-29 ~11:33 |
| `zumba-postgres-backup` | `0 1 * * 5` | Fri 2026-08-28 01:00 | Sat 2026-08-29 ~15:34 |

Both runs were skipped without any event or error — the controller never created
a Job, so there is nothing in `kubectl get jobs` to notice.

`wait-for-time.conf` prevents this by ordering k3s after `time-sync.target`, so
the CronJob controller only ever sees a synchronized clock.

`timeout.conf` is the safety net: upstream ships
`systemd-time-wait-sync.service` with `TimeoutStartSec=infinity`. Without it, a
boot without network or reachable NTP would block forever and k3s would never
start. With a 180s bound the unit fails, `time-sync.target` still completes, and
k3s starts anyway (`Wants=` is a soft dependency). Worst case is the old
behaviour, not a hung boot.

---

## Install

```bash
rsync -a --rsync-path="sudo rsync" \
  zumba-bot/deployment/node/k3s.service.d \
  zumba-bot/deployment/node/systemd-time-wait-sync.service.d \
  pi@192.168.178.46:/etc/systemd/system/

ssh pi@192.168.178.46 '
  sudo systemctl daemon-reload
  sudo systemctl enable systemd-time-wait-sync.service
'
```

No k3s restart required — the ordering takes effect on the next boot.

## Verify

```bash
ssh pi@192.168.178.46 '
  systemctl is-enabled systemd-time-wait-sync.service
  systemctl show k3s -p After -p Wants | tr " " "\n" | grep time-sync
  systemctl show systemd-time-wait-sync.service -p TimeoutStartUSec
'
```

Expected:

```
enabled
time-sync.target        # from After=
time-sync.target        # from Wants=
TimeoutStartUSec=3min
```

## Recovering from a missed schedule

The drop-in only helps on future boots. If a reboot already shifted the timers,
the queued delays stay wrong until the controller reconciles the CronJob again.
Touching the object is enough — the informer's update handler enqueues it
immediately and the delay is recomputed against the correct clock:

```bash
kubectl -n zumba-staging patch cronjob zumba-whatsapp-bot-weekly \
  --type=merge -p '{"metadata":{"annotations":{"requeue":"now"}}}'
kubectl -n zumba-staging patch cronjob zumba-whatsapp-bot-weekly \
  --type=merge -p '{"metadata":{"annotations":{"requeue":null}}}'
```

A run that was already missed is *not* caught up — `startingDeadlineSeconds` has
long expired by then. Trigger it manually if needed:

```bash
kubectl -n zumba-staging create job weekly-manual-$(date +%s) \
  --from=cronjob/zumba-whatsapp-bot-weekly
```

Note that this sends a real message to the WhatsApp group.

## Uninstall

```bash
ssh pi@192.168.178.46 '
  sudo rm -rf /etc/systemd/system/k3s.service.d \
              /etc/systemd/system/systemd-time-wait-sync.service.d
  sudo systemctl daemon-reload
  sudo systemctl disable systemd-time-wait-sync.service
'
```
