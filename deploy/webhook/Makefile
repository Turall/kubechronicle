# Makefile for kubechronicle deployment

.PHONY: deploy undeploy verify build

# Default namespace
NAMESPACE ?= kubechronicle

# Deploy all resources
deploy:
	@echo "Deploying kubechronicle..."
	kubectl apply -f namespace.yaml
	kubectl apply -f serviceaccount.yaml
	kubectl apply -f rbac.yaml
	kubectl apply -f deployment.yaml
	kubectl apply -f service.yaml
	kubectl apply -f webhook.yaml
	@echo "Deployment complete. Waiting for pods to be ready..."
	kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=kubechronicle -n $(NAMESPACE) --timeout=120s

# Undeploy all resources
undeploy:
	@echo "Undeploying kubechronicle..."
	kubectl delete validatingwebhookconfiguration kubechronicle-webhook || true
	kubectl delete -f service.yaml || true
	kubectl delete -f deployment.yaml || true
	kubectl delete -f rbac.yaml || true
	kubectl delete -f serviceaccount.yaml || true
	kubectl delete -f namespace.yaml || true
	@echo "Undeployment complete"

# Verify deployment
verify:
	@echo "Verifying deployment..."
	@echo "Pods:"
	kubectl get pods -n $(NAMESPACE)
	@echo "\nService:"
	kubectl get svc -n $(NAMESPACE)
	@echo "\nWebhook Configuration:"
	kubectl get validatingwebhookconfiguration kubechronicle-webhook
	@echo "\nPod Logs (last 20 lines):"
	kubectl logs -n $(NAMESPACE) -l app.kubernetes.io/name=kubechronicle --tail=20

# Generate self-signed TLS certificates
generate-certs:
	@echo "Generating self-signed TLS certificates..."
	@chmod +x generate-certs.sh
	@./generate-certs.sh

# Generate TLS certificates (alternative method using Makefile directly)
generate-certs-direct:
	@echo "Generating TLS certificates..."
	@openssl req -x509 -newkey rsa:4096 -keyout tls.key -out tls.crt -days 365 -nodes \
		-subj "/CN=kubechronicle-webhook.$(NAMESPACE).svc" \
		-addext "subjectAltName=DNS:kubechronicle-webhook.$(NAMESPACE).svc,DNS:kubechronicle-webhook.$(NAMESPACE).svc.cluster.local,DNS:kubechronicle-webhook,DNS:localhost"
	@echo "Creating Kubernetes secret..."
	@kubectl create namespace $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	@kubectl create secret tls kubechronicle-webhook-tls \
		--cert=tls.crt \
		--key=tls.key \
		-n $(NAMESPACE) \
		--dry-run=client -o yaml | kubectl apply -f -
	@echo "✓ Certificates created successfully!"
	@echo "Local files: tls.crt, tls.key (you can delete these after deployment)"

# Create database secret
create-db-secret:
	@echo "Creating database secret..."
	@read -p "Enter database URL: " dburl; \
	kubectl create secret generic kubechronicle-database \
		--from-literal=url="$$dburl" \
		-n $(NAMESPACE) \
		--dry-run=client -o yaml | kubectl apply -f -
	@echo "Database secret created"

# View logs
logs:
	kubectl logs -n $(NAMESPACE) -l app.kubernetes.io/name=kubechronicle -f

# Test webhook
test:
	@echo "Creating test deployment to trigger webhook..."
	kubectl create deployment test-webhook --image=nginx --dry-run=client -o yaml | kubectl apply -f -
	@sleep 2
	@echo "Checking webhook logs..."
	kubectl logs -n $(NAMESPACE) -l app.kubernetes.io/name=kubechronicle --tail=10
	@echo "Cleaning up test deployment..."
	kubectl delete deployment test-webhook || true
