# 🚀 Quick Deployment Guide

This is a quick reference for deploying n8n to your k3s cluster using ArgoCD.

## Pre-Deployment Checklist

Before applying the ApplicationSet, ensure:

- [ ] **Helm v3 installed** (`helm version`)
- [ ] **ArgoCD running** (`kubectl get pods -n argocd`)
- [ ] **Sealed Secrets controller running** (`kubectl get pods -n kube-system | grep sealed-secrets`)
- [ ] **SealedSecrets generated** (4 files in `environments/{staging,production}/sealed-secrets/`)
- [ ] **Passwords backed up** (saved from `/tmp/n8n-secrets-backup.txt` to password manager)
- [ ] **Git repository created** (GitHub, GitLab, Gitea, etc.)
- [ ] **Code committed to git** (all `deployment/` files)
- [ ] **Git URL updated** in `argocd/applicationset.yaml` (replace `UPDATE_ME`)

## Step-by-Step Deployment

### 1. Update Git Repository URL

```bash
cd deployment/argocd

# Option A: Edit manually
vim applicationset.yaml
# Find: repoURL: UPDATE_ME
# Replace with: repoURL: https://github.com/your-username/your-repo.git

# Option B: Use sed
sed -i 's|UPDATE_ME|https://github.com/your-username/your-repo.git|g' applicationset.yaml

# Verify
grep "repoURL:" applicationset.yaml
```

### 2. Commit and Push to Git

```bash
cd /home/michael/code/ai/n8n

# Check status
git status

# Add all deployment files
git add deployment/

# Commit
git commit -m "Add n8n GitOps deployment with ArgoCD"

# Push to remote
git push origin main  # or your branch name
```

### 3. Apply ApplicationSet to ArgoCD

```bash
# Apply the ApplicationSet
kubectl apply -f deployment/argocd/applicationset.yaml -n argocd

# Verify ApplicationSet was created
kubectl get applicationsets -n argocd

# Check that Applications were generated
kubectl get applications -n argocd
# Expected: zumba-staging, zumba-production
```

### 4. Monitor Staging Deployment

```bash
# Watch application status
kubectl get application zumba-staging -n argocd -w

# Watch pods come up
kubectl get pods -n zumba-staging -w

# Expected pods:
# - zumba-n8n-stack-n8n-xxxxx (n8n deployment)
# - zumba-n8n-stack-postgres-0 (postgres statefulset)
```

### 5. Check Deployment Status

```bash
# Get all resources in staging
kubectl get all -n zumba-staging

# Check PVCs are bound
kubectl get pvc -n zumba-staging

# Check secrets were created from SealedSecrets
kubectl get secrets -n zumba-staging

# Check ingress
kubectl get ingressroute -n zumba-staging
```

### 6. View Logs (if issues)

```bash
# n8n logs
kubectl logs -n zumba-staging -l app.kubernetes.io/component=n8n -f

# PostgreSQL logs
kubectl logs -n zumba-staging -l app.kubernetes.io/component=postgres -f

# Sealed Secrets controller logs (if secrets not decrypting)
kubectl logs -n kube-system -l app.kubernetes.io/name=sealed-secrets -f
```

### 7. Access n8n UI

**Staging**: http://zumba-stage.pi.home

```bash
# Check if ingress is working
curl -I http://zumba-stage.pi.home

# Or open in browser
xdg-open http://zumba-stage.pi.home
```

### 8. Validate Staging Environment

Once staging is working:

- [ ] n8n UI loads successfully
- [ ] Can complete initial setup (create admin account)
- [ ] Can create a test workflow
- [ ] Can execute workflows
- [ ] Data persists after pod restart: `kubectl rollout restart deployment/zumba-n8n-stack-n8n -n zumba-staging`

### 9. Deploy Production

Once staging is validated:

```bash
# Check production application status
kubectl get application zumba-production -n argocd

# Monitor production deployment
kubectl get pods -n zumba-production -w

# Access production UI
# http://zumba.pi.home
```

## Common Commands

### ArgoCD

```bash
# Get all applications
kubectl get applications -n argocd

# Get application details
argocd app get zumba-staging

# Sync application manually
argocd app sync zumba-staging

# View application in UI
kubectl port-forward svc/argocd-server -n argocd 8080:443
# Open: https://localhost:8080
```

### Troubleshooting

```bash
# Application not syncing
argocd app get zumba-staging --refresh
kubectl describe application zumba-staging -n argocd

# Pods not starting
kubectl describe pod <pod-name> -n zumba-staging
kubectl logs <pod-name> -n zumba-staging

# Secrets not decrypting
kubectl get sealedsecrets -n zumba-staging
kubectl get secrets -n zumba-staging
kubectl logs -n kube-system deployment/sealed-secrets-controller

# PVC not binding
kubectl describe pvc -n zumba-staging
kubectl get storageclass
```

### Updating Configuration

```bash
# Update Helm values
vim deployment/environments/staging/values.yaml
git add deployment/environments/staging/values.yaml
git commit -m "Update staging configuration"
git push

# ArgoCD will auto-sync within 3 minutes
# Or sync immediately:
argocd app sync zumba-staging
```

### Rotating Secrets

```bash
# Generate new secrets
cd deployment/scripts
./create-sealed-secret.sh staging postgres-secrets \
  POSTGRES_PASSWORD=$(openssl rand -base64 16) \
  DB_POSTGRESDB_PASSWORD=$(openssl rand -base64 16)

# Commit and push
git add ../environments/staging/sealed-secrets/
git commit -m "Rotate PostgreSQL password for staging"
git push

# Restart pods to pick up new secrets
kubectl rollout restart statefulset/zumba-n8n-stack-postgres -n zumba-staging
kubectl rollout restart deployment/zumba-n8n-stack-n8n -n zumba-staging
```

## Success Criteria

### Staging Environment ✅

- [ ] ArgoCD Application: `zumba-staging` shows "Synced" and "Healthy"
- [ ] Namespace: `zumba-staging` exists
- [ ] Pods: 2 pods running (n8n + postgres)
- [ ] PVCs: 2 PVCs bound
- [ ] Secrets: 2 secrets exist
- [ ] Ingress: IngressRoute configured
- [ ] UI: Accessible at http://zumba-stage.pi.home
- [ ] Functionality: Can create and execute workflows
- [ ] Persistence: Data survives pod restarts

### Production Environment ✅

- [ ] ArgoCD Application: `zumba-production` shows "Synced" and "Healthy"
- [ ] Namespace: `zumba-production` exists
- [ ] Pods: 2 pods running (n8n + postgres)
- [ ] PVCs: 2 PVCs bound (10Gi n8n, 20Gi postgres)
- [ ] Secrets: 2 secrets exist
- [ ] Ingress: IngressRoute configured
- [ ] UI: Accessible at http://zumba.pi.home
- [ ] Functionality: Can create and execute workflows
- [ ] Persistence: Data survives pod restarts

## Next Steps After Deployment

1. **Backup Strategy**: Set up PostgreSQL backups
2. **Monitoring**: Add Prometheus metrics and Grafana dashboards
3. **HTTPS/TLS**: Configure cert-manager and Let's Encrypt
4. **Resource Tuning**: Monitor actual usage and adjust requests/limits
5. **Alerting**: Set up alerts for pod failures, PVC usage, etc.

## Getting Help

- **ArgoCD Docs**: https://argo-cd.readthedocs.io/
- **n8n Docs**: https://docs.n8n.io/
- **Sealed Secrets**: https://github.com/bitnami-labs/sealed-secrets
- **Check deployment logs and events for detailed error messages**
