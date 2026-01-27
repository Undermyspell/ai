# n8n GitOps Deployment

Production-ready GitOps deployment of n8n workflow automation platform to k3s Kubernetes cluster using ArgoCD, Helm, and Kustomize.

## 📋 Overview

**What's Deployed:**
- ✅ n8n workflow automation platform (v2.5.0)
- ✅ PostgreSQL 18 database (persistent storage)
- ✅ Multi-environment (staging + production)
- ✅ GitOps workflow with ArgoCD
- ✅ Sealed Secrets for secure credential management
- ✅ Traefik ingress with custom domains

**Tech Stack:**
- **ArgoCD**: GitOps continuous delivery
- **Helm**: Application templating and versioning
- **Kustomize**: Environment-specific configuration
- **Sealed Secrets**: Encrypted secrets safe for git
- **k3s**: Lightweight Kubernetes on Raspberry Pi

---

## 🏗️ Architecture

```
GitHub (https://github.com/Undermyspell/ai)
  └── n8n/deployment/
      ├── helm-charts/zumba/          (Helm chart)
      ├── environments/
      │   ├── staging/values.yaml     (overrides + sealed-secrets)
      │   └── production/values.yaml  (overrides + sealed-secrets)
      └── argocd/applicationset.yaml
                ↓ git pull (every 3min)
ArgoCD ApplicationSet
  ├── zumba-staging   (namespace: zumba-staging)
  └── zumba-production (namespace: zumba-production)
                ↓ kubectl apply
Kubernetes Cluster (k3s)
  └── Per Environment:
      ├── Deployment: zumba-n8n (1 replica, port 5678)
      ├── StatefulSet: zumba-postgres (1 replica, port 5432)
      ├── Services: ClusterIP for both
      ├── PVCs: local-path storage
      ├── SealedSecrets → Secrets
      └── Traefik IngressRoute
                ↓
Users → http://zumba.pi.home (or zumba-stage.pi.home)
```

**Communication Flow:**
1. User → Traefik IngressRoute → Service `zumba-n8n:5678`
2. n8n → Service `zumba-postgres:5432` → PostgreSQL
3. Both mount PVCs for persistent data

---

## 📁 Directory Structure

```
deployment/
├── argocd/
│   └── applicationset.yaml          # Generates staging + production apps
├── helm-charts/zumba/               # Main Helm chart
│   ├── Chart.yaml                   # name: zumba, version: 0.1.0
│   ├── values.yaml                  # Default values
│   └── templates/                   # K8s manifests
│       ├── namespace.yaml
│       ├── n8n/                     # Deployment, Service, PVC, ConfigMap, IngressRoute
│       └── postgres/                # StatefulSet, Service, PVC, ConfigMap
├── environments/
│   ├── staging/
│   │   ├── values.yaml              # Staging overrides (2Gi storage)
│   │   ├── kustomization.yaml       # SealedSecrets reference
│   │   └── sealed-secrets/
│   │       ├── postgres-secrets.yaml
│   │       └── n8n-secrets.yaml
│   └── production/
│       ├── values.yaml              # Production overrides (4-5Gi storage)
│       ├── kustomization.yaml
│       └── sealed-secrets/
│           ├── postgres-secrets.yaml
│           └── n8n-secrets.yaml
└── scripts/
    ├── create-sealed-secret.sh      # Generate new SealedSecrets
    └── README.md
```

---

## ⚙️ Environment Configuration

| Setting | Staging | Production |
|---------|---------|------------|
| **Namespace** | `zumba-staging` | `zumba-production` |
| **n8n CPU** | 500m / 1 | 500m / 2 |
| **n8n Memory** | 500Mi / 1Gi | 1Gi / 2Gi |
| **n8n Storage** | 2Gi | 4Gi |
| **Postgres CPU** | 250m / 500m | 500m / 1 |
| **Postgres Memory** | 512Mi / 1Gi | 1Gi / 2Gi |
| **Postgres Storage** | 2Gi | 5Gi |
| **Ingress** | `http://zumba-stage.pi.home` | `http://zumba.pi.home` |
| **Storage Class** | `local-path` (k3s default) | `local-path` |

---

## 🚀 Quick Start

### Prerequisites

```bash
# Verify all components are running
kubectl get pods -n argocd              # ArgoCD
kubectl get pods -n kube-system | grep sealed-secrets  # Sealed Secrets
helm version                             # Helm v3.x
kubectl version                          # Client + Server
```

### Deploy to Cluster

```bash
# 1. Apply ApplicationSet
kubectl apply -f deployment/argocd/applicationset.yaml -n argocd

# 2. Watch deployment
kubectl get applications -n argocd -w
kubectl get pods -n zumba-staging -w

# 3. Access n8n
# Staging:    http://zumba-stage.pi.home
# Production: http://zumba.pi.home
```

### Verify Deployment

```bash
# Check sync status
kubectl get application zumba-staging -n argocd
# Expected: Synced, Healthy

# Check resources
kubectl get all,pvc,secrets,sealedsecrets -n zumba-staging

# Check logs
kubectl logs -n zumba-staging -l app.kubernetes.io/component=n8n -f
kubectl logs -n zumba-staging -l app.kubernetes.io/component=postgres -f
```

---

## 🔐 Secrets Management

### Current Secrets

Each environment has 2 encrypted SealedSecrets (safe to commit to git):
- `postgres-secrets`: PostgreSQL passwords
- `n8n-secrets`: n8n encryption key

**⚠️ Important:** Staging and production use **different** secrets!

### Generate New Secrets

```bash
cd deployment/scripts

# Example: Generate PostgreSQL secret for staging
./create-sealed-secret.sh staging postgres-secrets \
  POSTGRES_PASSWORD=$(openssl rand -base64 16) \
  DB_POSTGRESDB_PASSWORD=$(openssl rand -base64 16)

# Save passwords to password manager!
# Then commit and push
git add ../environments/staging/sealed-secrets/
git commit -m "Rotate PostgreSQL password"
git push

# Restart pods to pick up new secrets
kubectl rollout restart statefulset/zumba-postgres -n zumba-staging
kubectl rollout restart deployment/zumba-n8n -n zumba-staging
```

---

## 🔧 Common Operations

### Update Configuration

```bash
# Edit environment values
vim deployment/environments/staging/values.yaml

# Commit and push (ArgoCD auto-syncs within 3 minutes)
git add deployment/environments/staging/values.yaml
git commit -m "Increase staging resources"
git push
```

### Restart Pods

```bash
# Restart n8n
kubectl rollout restart deployment/zumba-n8n -n zumba-staging

# Restart PostgreSQL
kubectl rollout restart statefulset/zumba-postgres -n zumba-staging
```

### Check Logs

```bash
# n8n logs
kubectl logs -n zumba-staging -l app.kubernetes.io/component=n8n -f

# PostgreSQL logs
kubectl logs -n zumba-staging -l app.kubernetes.io/component=postgres -f

# ArgoCD Application events
kubectl describe application zumba-staging -n argocd
```

### ArgoCD UI

```bash
# Get admin password
kubectl get secret argocd-initial-admin-secret -n argocd \
  -o jsonpath='{.data.password}' | base64 -d && echo

# Port-forward
kubectl port-forward svc/argocd-server -n argocd 8080:443

# Open: https://localhost:8080
# Username: admin, Password: <from above>
```

---

## 🐛 Troubleshooting

### Application OutOfSync

```bash
# Check status
kubectl get application zumba-staging -n argocd

# Force refresh and sync
kubectl patch application zumba-staging -n argocd --type merge \
  -p '{"metadata":{"annotations":{"argocd.argoproj.io/refresh":"hard"}}}'
```

### Pods Not Starting

```bash
# Describe pod
kubectl describe pod <pod-name> -n zumba-staging

# Check events
kubectl get events -n zumba-staging --sort-by='.lastTimestamp'

# Common causes:
# - Image pull errors
# - PVC not binding
# - Secrets not available
# - Resource limits too low
```

### Secrets Not Decrypting

```bash
# Check SealedSecrets controller
kubectl get pods -n kube-system | grep sealed-secrets
kubectl logs -n kube-system deployment/sealed-secrets-controller

# Check SealedSecrets in namespace
kubectl get sealedsecrets -n zumba-staging
kubectl describe sealedsecret postgres-secrets -n zumba-staging

# Verify Secrets were created
kubectl get secrets -n zumba-staging
```

### PVC Issues

```bash
# Check PVC status
kubectl get pvc -n zumba-staging

# If resizing PVCs (can only expand, not shrink):
# 1. Delete existing PVC (⚠️ LOSES DATA)
kubectl delete pvc zumba-n8n-data -n zumba-staging
# 2. Delete pod to trigger recreation
kubectl delete pod <pod-name> -n zumba-staging
# 3. New PVC will be created with new size
```

---

## 📚 Next Steps

### 1. Enable HTTPS/TLS (Recommended)
- Install cert-manager
- Configure Let's Encrypt certificates
- Update IngressRoutes for TLS
- Set `N8N_SECURE_COOKIE: "true"` in values.yaml

### 2. Setup Backups
- PostgreSQL backup CronJob (pg_dump to external storage)
- n8n workflow exports to git
- PVC snapshots (if storage class supports it)
- Test restore procedures

### 3. Add Monitoring
- Deploy Prometheus + Grafana
- Configure n8n and PostgreSQL metrics
- Set up alerts for pod failures, high resource usage
- Centralized logging (Loki or ELK)

### 4. Security Hardening
- Network Policies (restrict pod-to-pod traffic)
- Pod Security Standards
- Secret rotation schedule
- RBAC review

### 5. High Availability (Optional)
- Scale n8n replicas (requires queue mode with Redis)
- PostgreSQL HA with operator (CloudNativePG, Zalando)
- Multi-node k3s cluster
- Networked storage (NFS, Longhorn)

---

## 📖 Additional Documentation

- **scripts/README.md** - Detailed secrets management guide
- **n8n Documentation**: https://docs.n8n.io/
- **ArgoCD Documentation**: https://argo-cd.readthedocs.io/
- **Sealed Secrets**: https://github.com/bitnami-labs/sealed-secrets

---

## 🎯 Success Criteria

**Per Environment:**
- [ ] ArgoCD Application shows "Synced" and "Healthy"
- [ ] 2 pods running (n8n + postgres)
- [ ] 2 PVCs bound with correct sizes
- [ ] 2 Secrets created from SealedSecrets
- [ ] IngressRoute configured
- [ ] UI accessible via browser
- [ ] Can create and execute workflows
- [ ] Data persists after pod restarts
