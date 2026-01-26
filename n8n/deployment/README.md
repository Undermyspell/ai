# n8n GitOps Deployment

Production-ready GitOps deployment of n8n workflow automation platform to k3s Kubernetes cluster using ArgoCD, Helm, and Kustomize.

## 📋 Overview

This deployment provides:
- ✅ **n8n** workflow automation platform
- ✅ **PostgreSQL** database (persistent storage)
- ✅ **Multi-environment** support (staging + production)
- ✅ **GitOps** workflow with ArgoCD
- ✅ **Sealed Secrets** for secure credential management
- ✅ **Helm** for templating and version management
- ✅ **Kustomize** for environment-specific configuration
- ✅ **Traefik** ingress with custom domains

## 🏗️ Architecture

```
ArgoCD ApplicationSet (creates 2 Applications)
├── zumba-staging (namespace: zumba-staging)
│   ├── Helm Chart → n8n + PostgreSQL
│   └── Kustomize → SealedSecrets + Labels
└── zumba-production (namespace: zumba-production)
    ├── Helm Chart → n8n + PostgreSQL
    └── Kustomize → SealedSecrets + Labels
```

### Components

- **ArgoCD**: GitOps continuous delivery
- **Helm Chart**: `zumba-stack` (n8n + PostgreSQL)
- **Kustomize**: Environment-specific configuration and labels
- **Sealed Secrets**: Encrypted secrets safe for git
- **Traefik**: HTTP ingress (IngressRoute CRD)

## 📁 Directory Structure

```
deployment/
├── argocd/
│   ├── applicationset.yaml          # ArgoCD ApplicationSet (deploys both envs)
│   └── README.md                    # ArgoCD reference documentation
├── helm-charts/
│   └── zumba-stack/                 # Main Helm chart
│       ├── Chart.yaml               # Chart metadata
│       ├── values.yaml              # Default values
│       └── templates/               # Kubernetes manifests
│           ├── namespace.yaml
│           ├── n8n/                 # n8n resources
│           │   ├── deployment.yaml
│           │   ├── service.yaml
│           │   ├── pvc.yaml
│           │   ├── configmap.yaml
│           │   └── ingress-route.yaml
│           └── postgres/            # PostgreSQL resources
│               ├── statefulset.yaml
│               ├── service.yaml
│               ├── pvc.yaml
│               └── configmap.yaml
├── environments/
│   ├── components/
│   │   └── common-labels/           # Reusable Kustomize component
│   │       └── kustomization.yaml   # Common labels (zumba.io/*)
│   ├── staging/
│   │   ├── values.yaml              # Staging Helm overrides
│   │   ├── kustomization.yaml       # Staging config (includes common-labels)
│   │   └── sealed-secrets/          # Encrypted secrets
│   │       ├── postgres-secrets.yaml
│   │       └── n8n-secrets.yaml
│   └── production/
│       ├── values.yaml              # Production Helm overrides
│       ├── kustomization.yaml       # Production config (includes common-labels)
│       └── sealed-secrets/          # Encrypted secrets
│           ├── postgres-secrets.yaml
│           └── n8n-secrets.yaml
├── scripts/
│   ├── create-sealed-secret.sh      # Generate new SealedSecrets
│   └── README.md                    # Scripts documentation
└── DEPLOYMENT.md                    # Quick start deployment guide
```

## 🚀 Quick Start

### Prerequisites

- ✅ k3s Kubernetes cluster running
- ✅ ArgoCD installed (`kubectl get pods -n argocd`)
- ✅ Sealed Secrets controller installed (`kubectl get pods -n kube-system | grep sealed-secrets`)
- ✅ Helm v3 installed (`helm version`)
- ✅ `kubectl` configured to access cluster
- ✅ `kubeseal` CLI installed
- ✅ Git repository created and accessible

### Deployment Steps

1. **Update Git Repository URL**:
   ```bash
   cd argocd
   sed -i 's|UPDATE_ME|https://github.com/YOUR-USERNAME/YOUR-REPO.git|g' applicationset.yaml
   ```

2. **Commit and Push**:
   ```bash
   git add .
   git commit -m "Add n8n GitOps deployment"
   git push origin main
   ```

3. **Apply ApplicationSet**:
   ```bash
   kubectl apply -f argocd/applicationset.yaml -n argocd
   ```

4. **Monitor Deployment**:
   ```bash
   # Watch applications
   kubectl get applications -n argocd -w
   
   # Watch pods
   kubectl get pods -n zumba-staging -w
   kubectl get pods -n zumba-production -w
   ```

5. **Access n8n**:
   - Staging: http://zumba-stage.pi.home
   - Production: http://zumba.pi.home

**📖 See [DEPLOYMENT.md](DEPLOYMENT.md) for detailed step-by-step guide**

## 🔐 Secrets Management

### Current Secrets

**Staging** (`zumba-staging` namespace):
- `postgres-secrets`: PostgreSQL passwords
- `n8n-secrets`: n8n encryption key

**Production** (`zumba-production` namespace):
- `postgres-secrets`: PostgreSQL passwords (different from staging!)
- `n8n-secrets`: n8n encryption key (different from staging!)

### Generating New Secrets

```bash
cd scripts

# Generate new secret
./create-sealed-secret.sh staging my-secret \
  KEY1=value1 \
  KEY2=$(openssl rand -base64 16)

# Commit and push
git add ../environments/staging/sealed-secrets/
git commit -m "Add new secret"
git push
```

**📖 See [scripts/README.md](scripts/README.md) for detailed secrets guide**

## 🏷️ Labels

All resources get consistent labels for organization and filtering.

### Common Labels (applied to all resources via component)
- `zumba.io/application: zumba`
- `zumba.io/managed-by: argocd`
- `zumba.io/stack: n8n`

### Environment-Specific Labels
- `zumba.io/environment: stage` (staging)
- `zumba.io/environment: prod` (production)

### Standard Kubernetes Labels (from Helm)
- `app.kubernetes.io/name: n8n-stack`
- `app.kubernetes.io/instance: zumba`
- `app.kubernetes.io/component: n8n|postgres`
- `app.kubernetes.io/managed-by: Helm`

## ⚙️ Configuration

### Environment Differences

| Setting | Staging | Production |
|---------|---------|------------|
| Namespace | `zumba-staging` | `zumba-production` |
| n8n CPU | 500m / 1000m | 1000m / 2000m |
| n8n Memory | 1Gi / 2Gi | 2Gi / 4Gi |
| n8n Storage | 2Gi | 10Gi |
| Postgres CPU | 250m / 500m | 500m / 1000m |
| Postgres Memory | 512Mi / 1Gi | 1Gi / 2Gi |
| Postgres Storage | 2Gi | 20Gi |
| Ingress Host | zumba-stage.pi.home | zumba.pi.home |
| Secure Cookie | false | false (true when HTTPS enabled) |

### Updating Configuration

**Helm Values** (resources, storage, etc.):
```bash
vim environments/staging/values.yaml
git add environments/staging/values.yaml
git commit -m "Update staging resources"
git push
# ArgoCD auto-syncs within 3 minutes
```

**Chart Defaults** (images, common settings):
```bash
vim helm-charts/zumba-stack/values.yaml
git add helm-charts/zumba-stack/values.yaml
git commit -m "Update n8n version"
git push
```

## 📊 Monitoring

### Check Application Status

```bash
# All applications
kubectl get applications -n argocd

# Specific application details
argocd app get zumba-staging

# Application resources
kubectl get all -n zumba-staging
```

### Check Pod Logs

```bash
# n8n logs
kubectl logs -n zumba-staging -l app.kubernetes.io/component=n8n -f

# PostgreSQL logs
kubectl logs -n zumba-staging -l app.kubernetes.io/component=postgres -f
```

### ArgoCD UI

```bash
# Get admin password
kubectl get secret argocd-initial-admin-secret -n argocd -o jsonpath='{.data.password}' | base64 -d

# Port-forward
kubectl port-forward svc/argocd-server -n argocd 8080:443

# Open https://localhost:8080
# Username: admin
# Password: <from above>
```

## 🔧 Common Operations

### Restart Pods

```bash
# Restart n8n (staging)
kubectl rollout restart deployment/zumba-n8n-stack-n8n -n zumba-staging

# Restart PostgreSQL (staging)
kubectl rollout restart statefulset/zumba-n8n-stack-postgres -n zumba-staging
```

### Scale n8n

```bash
# Edit values
vim environments/staging/values.yaml
# Change replicas or resources

# Commit and push
git commit -am "Scale n8n in staging"
git push
```

### Rotate Secrets

```bash
cd scripts

# Generate new PostgreSQL password
./create-sealed-secret.sh production postgres-secrets \
  POSTGRES_PASSWORD=$(openssl rand -base64 16) \
  DB_POSTGRESDB_PASSWORD=$(openssl rand -base64 16)

# Save password to password manager!

# Commit and push
git add ../environments/production/sealed-secrets/
git commit -m "Rotate PostgreSQL password"
git push

# Restart pods
kubectl rollout restart statefulset/zumba-n8n-stack-postgres -n zumba-production
kubectl rollout restart deployment/zumba-n8n-stack-n8n -n zumba-production
```

### Add New Environment

```bash
# 1. Copy staging environment
cp -r environments/staging environments/dev

# 2. Update values
vim environments/dev/values.yaml
# Change namespace, resources, etc.

# 3. Generate secrets
cd scripts
./create-sealed-secret.sh dev postgres-secrets \
  POSTGRES_PASSWORD=$(openssl rand -base64 16) \
  DB_POSTGRESDB_PASSWORD=$(openssl rand -base64 16)

./create-sealed-secret.sh dev n8n-secrets \
  N8N_ENCRYPTION_KEY=$(openssl rand -hex 16)

# 4. Update ApplicationSet
vim argocd/applicationset.yaml
# Add dev to generators list

# 5. Commit and apply
git add .
git commit -m "Add dev environment"
git push
kubectl apply -f argocd/applicationset.yaml -n argocd
```

## 🐛 Troubleshooting

### Application Stuck OutOfSync

```bash
# Check sync status
argocd app get zumba-staging

# Common causes:
# - Git URL not updated (still UPDATE_ME)
# - Helm chart errors
# - Invalid Kustomize syntax

# Force refresh
argocd app get zumba-staging --refresh
argocd app sync zumba-staging
```

### Pods Not Starting

```bash
# Describe pod
kubectl describe pod <pod-name> -n zumba-staging

# Common causes:
# - Image pull errors
# - PVC not binding
# - Secrets not available
# - Resource limits too low

# Check events
kubectl get events -n zumba-staging --sort-by='.lastTimestamp'
```

### Secrets Not Decrypting

```bash
# Check sealed-secrets controller
kubectl get pods -n kube-system | grep sealed-secrets
kubectl logs -n kube-system deployment/sealed-secrets-controller

# Check SealedSecrets
kubectl get sealedsecrets -n zumba-staging
kubectl describe sealedsecret postgres-secrets -n zumba-staging

# Regenerate if needed
cd scripts
./create-sealed-secret.sh staging postgres-secrets \
  POSTGRES_PASSWORD=$(openssl rand -base64 16) \
  DB_POSTGRESDB_PASSWORD=$(openssl rand -base64 16)
```

## 📚 Documentation

- **[DEPLOYMENT.md](DEPLOYMENT.md)** - Quick start deployment guide
- **[argocd/README.md](argocd/README.md)** - ArgoCD ApplicationSet reference
- **[scripts/README.md](scripts/README.md)** - Secrets management guide

## 🎯 Next Steps

After successful deployment:

1. **Enable HTTPS/TLS**
   - Install cert-manager
   - Configure Let's Encrypt
   - Update IngressRoutes for TLS
   - Set `N8N_SECURE_COOKIE: "true"`

2. **Setup Backups**
   - PostgreSQL backup CronJob
   - n8n workflow exports
   - PVC snapshots

3. **Add Monitoring**
   - Prometheus metrics
   - Grafana dashboards
   - AlertManager rules

4. **Resource Optimization**
   - Monitor actual usage
   - Adjust requests/limits
   - Consider HPA (Horizontal Pod Autoscaler)

5. **Network Policies**
   - Restrict pod-to-pod communication
   - Limit egress traffic

## 🤝 Contributing

To modify this deployment:

1. Create feature branch
2. Make changes
3. Test in staging environment
4. Create pull request
5. After approval, deploy to production

## 📝 License

This deployment configuration is provided as-is for use with n8n.

## 🔗 Links

- **n8n Documentation**: https://docs.n8n.io/
- **ArgoCD Documentation**: https://argo-cd.readthedocs.io/
- **Sealed Secrets**: https://github.com/bitnami-labs/sealed-secrets
- **Helm Documentation**: https://helm.sh/docs/
- **Kustomize Documentation**: https://kustomize.io/
