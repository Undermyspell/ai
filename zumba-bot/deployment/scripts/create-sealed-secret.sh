#!/bin/bash
#
# create-sealed-secret.sh - Generate and seal Kubernetes secrets
#
# Usage:
#   ./create-sealed-secret.sh <environment> <secret-name> [key=value ...]
#
# Examples:
#   # Create a new secret with manual values
#   ./create-sealed-secret.sh staging my-api-secret API_KEY=abc123 API_URL=https://example.com
#
#   # Create a secret with auto-generated password
#   ./create-sealed-secret.sh production db-backup-secret PASSWORD=$(openssl rand -base64 16)
#
#   # Regenerate existing n8n secrets
#   ./create-sealed-secret.sh staging n8n-secrets N8N_ENCRYPTION_KEY=$(openssl rand -hex 16)
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script directory and project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Configuration
SEALED_SECRETS_CONTROLLER_NAMESPACE="kube-system"

# Functions
usage() {
    echo "Usage: $0 <environment> <secret-name> [key=value ...]"
    echo ""
    echo "Arguments:"
    echo "  environment    - Environment name (staging, production, or custom)"
    echo "  secret-name    - Name of the Kubernetes secret"
    echo "  key=value      - One or more key-value pairs (space-separated)"
    echo ""
    echo "Examples:"
    echo "  $0 staging postgres-secrets POSTGRES_PASSWORD=\$(openssl rand -base64 16)"
    echo "  $0 production api-keys API_KEY=abc123 API_SECRET=xyz789"
    echo ""
    echo "Helper commands for generating secure values:"
    echo "  openssl rand -base64 16    # 16-byte password (base64)"
    echo "  openssl rand -hex 16       # 32-character hex string"
    echo "  openssl rand -base64 32    # 32-byte password (base64)"
    echo ""
    exit 1
}

error() {
    echo -e "${RED}ERROR: $1${NC}" >&2
    exit 1
}

info() {
    echo -e "${BLUE}INFO: $1${NC}"
}

success() {
    echo -e "${GREEN}✅ $1${NC}"
}

warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

# Check prerequisites
check_prerequisites() {
    info "Checking prerequisites..."
    
    if ! command -v kubectl &> /dev/null; then
        error "kubectl is not installed or not in PATH"
    fi
    
    if ! command -v kubeseal &> /dev/null; then
        error "kubeseal is not installed or not in PATH"
    fi
    
    # Check if sealed secrets controller is running
    if ! kubectl get deployment sealed-secrets-controller -n "$SEALED_SECRETS_CONTROLLER_NAMESPACE" &> /dev/null; then
        error "Sealed Secrets controller not found in namespace: $SEALED_SECRETS_CONTROLLER_NAMESPACE"
    fi
    
    success "Prerequisites check passed"
}

# Parse arguments
if [ $# -lt 3 ]; then
    usage
fi

ENVIRONMENT="$1"
SECRET_NAME="$2"
shift 2

# Determine namespace
case "$ENVIRONMENT" in
    staging)
        NAMESPACE="zumba-staging"
        ENV_LABEL="stage"
        ;;
    production)
        NAMESPACE="zumba-production"
        ENV_LABEL="prod"
        ;;
    *)
        # Custom environment
        NAMESPACE="zumba-$ENVIRONMENT"
        ENV_LABEL="$ENVIRONMENT"
        warning "Using custom environment: $ENVIRONMENT (namespace: $NAMESPACE)"
        ;;
esac

# Parse key-value pairs
declare -a LITERALS
for arg in "$@"; do
    if [[ $arg == *"="* ]]; then
        LITERALS+=("--from-literal=$arg")
    else
        error "Invalid argument: $arg (must be key=value format)"
    fi
done

if [ ${#LITERALS[@]} -eq 0 ]; then
    error "At least one key=value pair is required"
fi

# Output file path
OUTPUT_DIR="$PROJECT_ROOT/deployment/environments/$ENVIRONMENT/sealed-secrets"
OUTPUT_FILE="$OUTPUT_DIR/${SECRET_NAME}.yaml"

# Create output directory if it doesn't exist
mkdir -p "$OUTPUT_DIR"

info "Environment: $ENVIRONMENT"
info "Namespace: $NAMESPACE"
info "Secret Name: $SECRET_NAME"
info "Output File: $OUTPUT_FILE"
echo ""

# Check prerequisites
check_prerequisites
echo ""

# Display the secret keys (not values) that will be created
info "Secret will contain the following keys:"
for literal in "${LITERALS[@]}"; do
    KEY=$(echo "$literal" | sed 's/--from-literal=//' | cut -d'=' -f1)
    echo "  - $KEY"
done
echo ""

# Confirm before proceeding (if file exists)
if [ -f "$OUTPUT_FILE" ]; then
    warning "File already exists: $OUTPUT_FILE"
    read -p "Overwrite? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        info "Aborted by user"
        exit 0
    fi
fi

# Create the sealed secret
info "Creating sealed secret..."

kubectl create secret generic "$SECRET_NAME" \
    --namespace="$NAMESPACE" \
    "${LITERALS[@]}" \
    --dry-run=client -o yaml | \
kubeseal --controller-namespace="$SEALED_SECRETS_CONTROLLER_NAMESPACE" --format=yaml > "$OUTPUT_FILE"

if [ $? -eq 0 ]; then
    success "Sealed secret created successfully!"
    echo ""
    info "File saved to: $OUTPUT_FILE"
    info "File size: $(du -h "$OUTPUT_FILE" | cut -f1)"
    echo ""
    
    # Show snippet of the file
    info "Preview (first 20 lines):"
    head -20 "$OUTPUT_FILE"
    echo ""
    
    success "Next steps:"
    echo "  1. Verify the sealed secret: kubectl kustomize $OUTPUT_DIR/.."
    echo "  2. Commit to git: git add $OUTPUT_FILE"
    echo "  3. Deploy via ArgoCD or apply directly"
    echo ""
    warning "IMPORTANT: Save the plaintext values to your password manager!"
    echo "          The sealed secret file only contains encrypted data."
else
    error "Failed to create sealed secret"
fi
