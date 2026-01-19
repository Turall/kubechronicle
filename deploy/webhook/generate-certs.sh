#!/bin/bash
# Generate self-signed TLS certificates for kubechronicle webhook
# This script creates a self-signed certificate suitable for internal cluster use

set -e

NAMESPACE="${NAMESPACE:-kubechronicle}"
SERVICE_NAME="kubechronicle-webhook"
SECRET_NAME="kubechronicle-webhook-tls"
DAYS_VALID="${DAYS_VALID:-365}"

# Generate certificate
echo "Generating self-signed certificate for ${SERVICE_NAME}..."
openssl req -x509 -newkey rsa:4096 \
  -keyout tls.key \
  -out tls.crt \
  -days ${DAYS_VALID} \
  -nodes \
  -subj "/CN=${SERVICE_NAME}.${NAMESPACE}.svc" \
  -addext "subjectAltName=DNS:${SERVICE_NAME}.${NAMESPACE}.svc,DNS:${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local,DNS:${SERVICE_NAME},DNS:localhost"

# Create or update Kubernetes secret
echo "Creating Kubernetes secret ${SECRET_NAME} in namespace ${NAMESPACE}..."

# Check if namespace exists, create if not
if ! kubectl get namespace "${NAMESPACE}" &>/dev/null; then
  echo "Creating namespace ${NAMESPACE}..."
  kubectl create namespace "${NAMESPACE}"
fi

# Create or update the secret
kubectl create secret tls "${SECRET_NAME}" \
  --cert=tls.crt \
  --key=tls.key \
  --namespace="${NAMESPACE}" \
  --dry-run=client -o yaml | kubectl apply -f -

# Base64 encode the certificate for caBundle
CA_BUNDLE=$(base64 -w 0 < tls.crt 2>/dev/null || base64 < tls.crt | tr -d '\n')

# Update local webhook.yaml if it exists
WEBHOOK_YAML="webhook.yaml"
if [ -f "${WEBHOOK_YAML}" ]; then
  echo "Updating local ${WEBHOOK_YAML} with CA bundle..."
  # Check if caBundle field exists, if not add it
  if grep -q "caBundle:" "${WEBHOOK_YAML}"; then
    # Update existing caBundle (works with both macOS and Linux sed)
    if [[ "$OSTYPE" == "darwin"* ]]; then
      sed -i '' "s|caBundle:.*|caBundle: ${CA_BUNDLE}|" "${WEBHOOK_YAML}"
    else
      sed -i "s|caBundle:.*|caBundle: ${CA_BUNDLE}|" "${WEBHOOK_YAML}"
    fi
  else
    # Add caBundle field after path
    # Use awk with temp file for cross-platform compatibility
    TEMP_FILE=$(mktemp)
    awk -v ca_bundle="${CA_BUNDLE}" '/path: \/validate/ {print; print "    caBundle: " ca_bundle; next} {print}' "${WEBHOOK_YAML}" > "${TEMP_FILE}"
    mv "${TEMP_FILE}" "${WEBHOOK_YAML}"
  fi
  echo "✓ Local ${WEBHOOK_YAML} updated with CA bundle"
fi

# Update ValidatingWebhookConfiguration in cluster if it exists
WEBHOOK_CONFIG="kubechronicle-webhook"
if kubectl get validatingwebhookconfiguration "${WEBHOOK_CONFIG}" &>/dev/null; then
  echo "Updating ValidatingWebhookConfiguration ${WEBHOOK_CONFIG} in cluster with CA bundle..."
  kubectl patch validatingwebhookconfiguration "${WEBHOOK_CONFIG}" \
    --type=json \
    -p="[{\"op\": \"replace\", \"path\": \"/webhooks/0/clientConfig/caBundle\", \"value\": \"${CA_BUNDLE}\"}]" 2>/dev/null || \
  kubectl patch validatingwebhookconfiguration "${WEBHOOK_CONFIG}" \
    --type=merge \
    -p="{\"webhooks\":[{\"name\":\"kubechronicle.k8s.io\",\"clientConfig\":{\"caBundle\":\"${CA_BUNDLE}\"}}]}"
  echo "✓ Cluster webhook configuration updated with CA bundle"
else
  echo "⚠ ValidatingWebhookConfiguration ${WEBHOOK_CONFIG} not found in cluster."
  echo "   Deploy the webhook first, then run this script again to update the CA bundle."
fi

echo ""
echo "✓ Certificate secret created successfully!"
echo ""
echo "Certificate details:"
echo "  Secret name: ${SECRET_NAME}"
echo "  Namespace: ${NAMESPACE}"
echo "  Valid for: ${DAYS_VALID} days"
echo ""
echo "Local certificate files:"
echo "  tls.crt - Certificate"
echo "  tls.key - Private key"
echo ""
echo "You can now deploy kubechronicle. The secret is ready to use."
echo ""
echo "To clean up local files (optional):"
echo "  rm tls.crt tls.key"
