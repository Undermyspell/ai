# ArgoCD ApplicationSet

This directory contains the ArgoCD ApplicationSet that deploys n8n to staging and production environments.

## Before Deployment

**IMPORTANT**: Update the git repository URL before applying this ApplicationSet!

### Update Git Repository URL

1. **Find the placeholder** in `applicationset.yaml`:
   ```yaml
   repoURL: UPDATE_ME  # Line appears twice (once per source)
   ```

2. **Replace with your actual git repository URL**:
   ```yaml
   # Example for GitHub
   repoURL: https://github.com/your-username/your-repo.git
   
   # Example for GitLab
   repoURL: https://gitlab.com/your-username/your-repo.git
   
   # Example for self-hosted Gitea
   repoURL: https://git.yourdomain.com/your-username/your-repo.git
   ```

3. **Using sed for quick replacement**:
   ```bash
   # Replace UPDATE_ME with your actual URL (both occurrences)
   sed -i 's|UPDATE_ME|https://github.com/your-username/your-repo.git|g' applicationset.yaml
   ```

## What This ApplicationSet Does

Creates two ArgoCD Applications:
- `zumba-staging` - Deploys to `zumba-staging` namespace
- `zumba-production` - Deploys to `zumba-production` namespace

Each application uses **multi-source deployment**:

### Source 1: Helm Chart
- **Path**: `n8n/deployment/helm-charts/zumba-stack`
- **Purpose**: Main application resources (n8n Deployment, PostgreSQL StatefulSet, Services, PVCs, etc.)
- **Values**: Environment-specific values from `environments/{staging|production}/values.yaml`

### Source 2: Kustomize
- **Path**: `n8n/deployment/environments/{staging|production}`
- **Purpose**: SealedSecrets with labels
- **Resources**: 
  - `sealed-secrets/postgres-secrets.yaml`
  - `sealed-secrets/n8n-secrets.yaml`
- **Labels**: Includes `components/common-labels` + environment-specific labels

## Sync Policy

**Automated Sync**: Enabled
- **Prune**: Resources removed from git are deleted from cluster
- **Self-Heal**: Cluster state automatically corrected if it drifts from git

**Sync Options**:
- **CreateNamespace**: Namespaces auto-created if they don't exist
- **ServerSideApply**: Better conflict resolution
- **Retry**: Up to 5 retries with exponential backoff (5s → 10s → 20s → 40s → 3m)

## Deployment

### Prerequisites

1. ✅ ArgoCD installed and running
2. ✅ Git repository URL updated in `applicationset.yaml`
3. ✅ SealedSecrets generated (see `../scripts/README.md`)
4. ✅ Code committed and pushed to git

### Apply the ApplicationSet

```bash
# From the argocd directory
kubectl apply -f applicationset.yaml -n argocd
```

### Verify Applications Created

```bash
# List applications
kubectl get applications -n argocd

# Expected output:
# NAME              SYNC STATUS   HEALTH STATUS
# zumba-staging     Synced        Healthy
# zumba-production  OutOfSync     Healthy
```

### Sync Applications

**Option 1: Let automated sync happen** (wait ~3 minutes)

**Option 2: Manual sync via CLI**:
```bash
# Sync staging
argocd app sync zumba-staging

# Sync production
argocd app sync zumba-production
```

**Option 3: Manual sync via UI**:
1. Open ArgoCD UI
2. Click on `zumba-staging`
3. Click "SYNC" button
4. Repeat for `zumba-production`

## Monitoring Deployment

### Watch Application Status

```bash
# Watch all applications
kubectl get applications -n argocd -w

# Get detailed status for staging
kubectl get application zumba-staging -n argocd -o yaml

# Get detailed status for production
kubectl get application zumba-production -n argocd -o yaml
```

### Watch Pods

```bash
# Staging pods
kubectl get pods -n zumba-staging -w

# Production pods
kubectl get pods -n zumba-production -w
```

### Check Logs

```bash
# n8n logs (staging)
kubectl logs -n zumba-staging deployment/zumba-n8n-stack-n8n -f

# PostgreSQL logs (staging)
kubectl logs -n zumba-staging statefulset/zumba-n8n-stack-postgres -f
```

### View in ArgoCD UI

```bash
# Get ArgoCD admin password
kubectl get secret argocd-initial-admin-secret -n argocd -o jsonpath='{.data.password}' | base64 -d

# Port-forward ArgoCD UI
kubectl port-forward svc/argocd-server -n argocd 8080:443

# Open in browser
# https://localhost:8080
# Username: admin
# Password: <from above command>
```

## Troubleshooting

### Application Stuck in "OutOfSync"

**Check sync status**:
```bash
argocd app get zumba-staging
```

**Common causes**:
- Git repository URL not updated (still shows `UPDATE_ME`)
- Git repository not accessible (private repo without credentials)
- Invalid path in git repository
- Helm chart errors

**Solution**:
```bash
# Update git URL in applicationset.yaml
# Then reapply
kubectl apply -f applicationset.yaml -n argocd

# Force refresh
argocd app get zumba-staging --refresh
```

### Application Shows "Degraded" Health

**Check resource status**:
```bash
kubectl get all -n zumba-staging
```

**Common causes**:
- Pods not starting (check `kubectl describe pod`)
- PVCs not binding (check storage class exists)
- Secrets not decrypted (check sealed-secrets controller logs)

### SealedSecrets Not Decrypting

**Check sealed-secrets controller**:
```bash
kubectl get pods -n kube-system | grep sealed-secrets
kubectl logs -n kube-system deployment/sealed-secrets-controller
```

**Verify SealedSecret exists**:
```bash
kubectl get sealedsecrets -n zumba-staging
kubectl get secrets -n zumba-staging
```

**Common causes**:
- Sealed secret encrypted for wrong namespace
- Sealed secret encrypted with different cluster keys
- Sealed-secrets controller not running

**Solution**: Regenerate secrets using `../scripts/create-sealed-secret.sh`

### Multi-Source Support Error

If you see errors about multi-source:

**Check ArgoCD version**:
```bash
argocd version
```

**Requirement**: ArgoCD v2.6.0+ for multi-source support

**Upgrade if needed**:
```bash
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
```

## Configuration Reference

### Labels Applied to Applications

Each ArgoCD Application gets these labels:
- `app.kubernetes.io/name: n8n-stack`
- `app.kubernetes.io/instance: zumba-{staging|production}`
- `app.kubernetes.io/managed-by: argocd`
- `zumba.io/application: zumba`
- `zumba.io/environment: staging|production`

### Templating

Uses **Go templates** with `goTemplate: true`:
- `{{.env}}` - Environment name (staging, production)
- `{{.namespace}}` - Namespace name (zumba-staging, zumba-production)

### Adding a New Environment

To add a "dev" environment:

1. **Update ApplicationSet**:
   ```yaml
   generators:
   - list:
       elements:
       - env: staging
         namespace: zumba-staging
       - env: production
         namespace: zumba-production
       - env: dev          # Add this
         namespace: zumba-dev  # Add this
   ```

2. **Create environment directory**:
   ```bash
   cp -r environments/staging environments/dev
   ```

3. **Update dev values.yaml** with appropriate settings

4. **Generate dev secrets**:
   ```bash
   cd ../scripts
   ./create-sealed-secret.sh dev postgres-secrets \
     POSTGRES_PASSWORD=$(openssl rand -base64 16) \
     DB_POSTGRESDB_PASSWORD=$(openssl rand -base64 16)
   
   ./create-sealed-secret.sh dev n8n-secrets \
     N8N_ENCRYPTION_KEY=$(openssl rand -hex 16)
   ```

5. **Commit and apply**:
   ```bash
   git add .
   git commit -m "Add dev environment"
   git push
   kubectl apply -f applicationset.yaml -n argocd
   ```

## Files

```
argocd/
├── applicationset.yaml  - ApplicationSet manifest
└── README.md           - This file
```

## Next Steps

After successful deployment:

1. ✅ Verify pods are running
2. ✅ Check services and ingress
3. ✅ Access n8n UI via ingress host
4. ✅ Complete n8n initial setup (create admin user)
5. ✅ Test workflow creation and execution
6. ✅ Verify data persistence (restart pods)
7. ⏭️ Enable HTTPS/TLS (future enhancement)
