# Building the UI

## Important: Rebuild Required

After adding the login page, you **must rebuild the UI Docker image** for the changes to take effect.

## Build Commands

### For Kubernetes Deployment

```bash
cd ui

# Build with correct API URL for Kubernetes
docker build \
  --build-arg VITE_API_URL=http://kubechronicle-api:80 \
  -t kubechronicle/ui:latest .

# Or for your registry
docker build \
  --build-arg VITE_API_URL=http://kubechronicle-api:80 \
  -t registry.digitalocean.com/kubechronicle/ui:latest .

# Push to registry
docker push registry.digitalocean.com/kubechronicle/ui:latest
```

### For Local Development

```bash
cd ui

# Build with localhost API URL
docker build \
  --build-arg VITE_API_URL=http://localhost:8080 \
  -t kubechronicle/ui:latest .
```

## Verify Login Page is Included

After building, verify the login page is in the build:

```bash
# Check if login route exists in built files
docker run --rm kubechronicle/ui:latest cat /usr/share/nginx/html/index.html | grep -i login

# Or extract and check
docker create --name temp kubechronicle/ui:latest
docker cp temp:/usr/share/nginx/html ./ui-dist
docker rm temp
ls -la ui-dist/
```

## After Rebuilding

1. **Update deployment**:
   ```bash
   kubectl set image deployment/kubechronicle-ui \
     ui=kubechronicle/ui:latest \
     -n kubechronicle
   ```

2. **Or apply updated deployment**:
   ```bash
   kubectl apply -f deploy/ui/deployment.yaml
   ```

3. **Verify**:
   ```bash
   kubectl get pods -n kubechronicle -l app.kubernetes.io/component=ui
   kubectl logs -n kubechronicle -l app.kubernetes.io/component=ui
   ```

## Troubleshooting

If login page still doesn't show:

1. **Clear browser cache**: Hard refresh (Ctrl+Shift+R or Cmd+Shift+R)
2. **Check browser console**: Look for JavaScript errors
3. **Verify route**: Navigate directly to `/login`
4. **Check API connectivity**: Verify UI can reach API service
