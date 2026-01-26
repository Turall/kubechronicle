# Building the UI

## Important: Rebuild Required

After adding the login page, you **must rebuild the UI Docker image** for the changes to take effect.

## Building with GitHub Actions

The GitHub Actions workflow (`.github/workflows/build-ui.yml`) supports passing the API URL in multiple ways:

### Option 1: Using GitHub Secrets (Recommended for Production)

1. Go to your repository **Settings → Secrets and variables → Actions**
2. Add a new secret named `VITE_API_URL` with your API URL (e.g., `http://kubechronicle-api:80`)
3. The workflow will automatically use this secret if it exists

### Option 2: Using Workflow Input

When manually triggering the workflow:
1. Go to **Actions → Build UI Container → Run workflow**
2. Fill in the `api_url` input field with your API URL (e.g., `http://kubechronicle-api:80`)
3. Optionally set a custom `tag` (defaults to `latest`)
4. Click **Run workflow**

### Option 3: Default Value

If neither secret nor input is provided, it defaults to `http://kubechronicle-api:80`

**Priority order**: `secrets.VITE_API_URL` → `github.event.inputs.api_url` → default value

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
