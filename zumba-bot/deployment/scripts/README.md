# Deployment Scripts

This directory contains helper scripts for managing the n8n deployment.

## Scripts

### `create-sealed-secret.sh`

Generate and seal Kubernetes secrets for any environment.

#### Prerequisites

- `kubectl` installed and configured
- `kubeseal` CLI installed
- Sealed Secrets controller running in the cluster (namespace: `kube-system`)

#### Usage

```bash
./create-sealed-secret.sh <environment> <secret-name> [key=value ...]
```

**Arguments:**
- `environment` - Environment name (`staging`, `production`, or custom)
- `secret-name` - Name of the Kubernetes secret to create
- `key=value` - One or more key-value pairs (space-separated)

#### Examples

**Generate new PostgreSQL secrets with random password:**
```bash
./create-sealed-secret.sh staging postgres-secrets \
  POSTGRES_PASSWORD=$(openssl rand -base64 16) \
  DB_POSTGRESDB_PASSWORD=$(openssl rand -base64 16)
```

**Generate n8n encryption key:**
```bash
./create-sealed-secret.sh production n8n-secrets \
  N8N_ENCRYPTION_KEY=$(openssl rand -hex 16)
```

**Create API credentials secret:**
```bash
./create-sealed-secret.sh staging external-api-secrets \
  API_KEY=your-api-key-here \
  API_SECRET=your-api-secret-here \
  API_URL=https://api.example.com
```

**Create secret for custom environment:**
```bash
./create-sealed-secret.sh dev my-custom-secret \
  KEY1=value1 \
  KEY2=value2
```

#### Helper Commands for Generating Secure Values

```bash
# 16-byte password (base64 encoded)
openssl rand -base64 16

# 32-character hex string (good for encryption keys)
openssl rand -hex 16

# 32-byte password (base64 encoded)
openssl rand -base64 32

# UUID
uuidgen
```

#### Output

The script will:
1. ✅ Check prerequisites (kubectl, kubeseal, sealed-secrets controller)
2. ✅ Create a Kubernetes Secret manifest (dry-run)
3. ✅ Encrypt it using kubeseal
4. ✅ Save to `../environments/<environment>/sealed-secrets/<secret-name>.yaml`
5. ✅ Display preview and next steps

#### Security Notes

- ⚠️ **IMPORTANT**: The script creates encrypted SealedSecrets, but you still need to save the plaintext values to your password manager!
- ✅ Sealed secrets are safe to commit to git (they're encrypted)
- ✅ Only your Kubernetes cluster can decrypt them (sealed to cluster's public key)
- ✅ Different environments use different namespaces, so secrets are isolated

#### Workflow

1. **Generate secret:**
   ```bash
   ./create-sealed-secret.sh staging my-secret PASSWORD=$(openssl rand -base64 16)
   ```

2. **Save plaintext password to password manager** (the output shows the file created, not the password!)

3. **Verify the sealed secret:**
   ```bash
   kubectl kustomize ../environments/staging
   ```

4. **Commit to git:**
   ```bash
   git add ../environments/staging/sealed-secrets/my-secret.yaml
   git commit -m "Add my-secret for staging environment"
   git push
   ```

5. **ArgoCD will automatically deploy** (if auto-sync is enabled)

## Environment Structure

Each environment has its own sealed-secrets directory:

```
environments/
├── staging/
│   ├── sealed-secrets/
│   │   ├── postgres-secrets.yaml
│   │   ├── n8n-secrets.yaml
│   │   └── <your-custom-secret>.yaml
│   ├── values.yaml
│   └── kustomization.yaml
└── production/
    ├── sealed-secrets/
    │   ├── postgres-secrets.yaml
    │   ├── n8n-secrets.yaml
    │   └── <your-custom-secret>.yaml
    ├── values.yaml
    └── kustomization.yaml
```

## Rotating Secrets

To rotate a secret (e.g., change PostgreSQL password):

1. **Generate new sealed secret** (overwrites existing):
   ```bash
   ./create-sealed-secret.sh production postgres-secrets \
     POSTGRES_PASSWORD=$(openssl rand -base64 16) \
     DB_POSTGRESDB_PASSWORD=$(openssl rand -base64 16)
   ```

2. **Save new password to password manager**

3. **Commit and push:**
   ```bash
   git add ../environments/production/sealed-secrets/postgres-secrets.yaml
   git commit -m "Rotate PostgreSQL password for production"
   git push
   ```

4. **ArgoCD syncs new secret to cluster**

5. **Restart pods to pick up new secret:**
   ```bash
   kubectl rollout restart statefulset/postgres -n zumba-production
   kubectl rollout restart deployment/n8n -n zumba-production
   ```

## Troubleshooting

**Error: "Sealed Secrets controller not found"**
- Check if controller is running: `kubectl get pods -n kube-system | grep sealed-secrets`
- If in different namespace, update `SEALED_SECRETS_CONTROLLER_NAMESPACE` in the script

**Error: "kubeseal: command not found"**
- Install kubeseal: https://github.com/bitnami-labs/sealed-secrets#installation

**Error: "cannot fetch certificate"**
- The sealed-secrets controller might not be ready
- Wait a few minutes and try again
- Check controller logs: `kubectl logs -n kube-system deployment/sealed-secrets-controller`

**Sealed secret not being decrypted in cluster**
- Check if namespace exists: `kubectl get namespace zumba-staging`
- Check sealed-secrets controller logs for errors
- Verify the sealed secret was created for the correct namespace
