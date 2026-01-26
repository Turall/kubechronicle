# UI Troubleshooting Guide

## Login Page Not Showing

If you don't see the login page when accessing the UI, check the following:

### 1. Rebuild the UI Docker Image

The login page code needs to be included in the Docker image:

```bash
cd ui
npm run build
docker build -t kubechronicle/ui:latest .
# Or push to your registry
docker push kubechronicle/ui:latest
```

### 2. Check API URL Configuration

The UI needs to know where the API is. Check:

- **Environment variable**: `VITE_API_URL` must be set at build time
- **Dockerfile**: The UI Dockerfile sets `ENV VITE_API_URL=http://localhost:8080` (update this)

**To fix:**
```dockerfile
# In ui/Dockerfile, update the API URL:
ENV VITE_API_URL=http://kubechronicle-api:80
```

Or build with build arg:
```bash
docker build --build-arg VITE_API_URL=http://kubechronicle-api:80 -t kubechronicle/ui:latest ./ui
```

### 3. Verify Authentication is Enabled

Check if authentication is enabled in the API deployment:

```bash
kubectl get deployment kubechronicle-api -n kubechronicle -o yaml | grep AUTH_ENABLED
```

Should show: `value: "true"`

### 4. Check Browser Console

Open browser developer tools (F12) and check:
- Console for errors
- Network tab for API requests
- Check if `/api/changes` returns 401 (auth required) or 200 (auth disabled)

### 5. Access Login Page Directly

Try accessing the login page directly:
```
http://your-ui-url/login
```

### 6. Clear Browser Cache

Clear localStorage and cookies:
```javascript
// In browser console:
localStorage.clear();
location.reload();
```

### 7. Check API Connectivity

Verify the UI can reach the API:

```bash
# Port forward to API
kubectl port-forward -n kubechronicle svc/kubechronicle-api 8080:80

# Test API
curl http://localhost:8080/api/changes?limit=1
# Should return 401 if auth is enabled
```

### 8. Verify Routes

Check that the login route is registered:

1. Open browser developer tools
2. Go to Network tab
3. Navigate to `/login`
4. Check if the route loads

### Common Issues

**Issue**: UI shows "Loading..." forever
- **Cause**: API check is failing (network/CORS)
- **Fix**: Check API URL configuration and network connectivity

**Issue**: UI allows access without login
- **Cause**: Auth check determined auth is disabled
- **Fix**: Verify `AUTH_ENABLED=true` in API deployment

**Issue**: Login page shows but login fails
- **Cause**: Auth secret not configured or wrong credentials
- **Fix**: Check `kubechronicle-auth` secret exists and has correct users

**Issue**: Redirect loop
- **Cause**: Login succeeds but token not stored
- **Fix**: Check browser console for errors, verify localStorage is accessible

## Debugging Steps

1. **Check API Response**:
   ```bash
   # Without auth (should return 401)
   curl http://api-url/api/changes?limit=1
   
   # With auth (should return 200)
   curl -H "Authorization: Bearer <token>" http://api-url/api/changes?limit=1
   ```

2. **Check UI Logs**:
   ```bash
   kubectl logs -n kubechronicle -l app.kubernetes.io/component=ui
   ```

3. **Check API Logs**:
   ```bash
   kubectl logs -n kubechronicle -l app.kubernetes.io/component=api | grep -i auth
   ```

4. **Verify Secrets**:
   ```bash
   kubectl get secret kubechronicle-auth -n kubechronicle
   kubectl get secret kubechronicle-auth -n kubechronicle -o jsonpath='{.data.users}' | base64 -d
   ```

5. **Test Login Endpoint**:
   ```bash
   curl -X POST http://api-url/api/auth/login \
     -H "Content-Type: application/json" \
     -d '{"username":"admin","password":"your-password"}'
   ```

## Quick Fixes

### Force Show Login Page

If you need to access login immediately, you can:

1. **Clear localStorage**:
   ```javascript
   // In browser console
   localStorage.removeItem('auth_token');
   localStorage.removeItem('auth_user');
   location.href = '/login';
   ```

2. **Access directly**: Navigate to `http://your-ui-url/login`

3. **Rebuild UI**: Make sure the latest code is built into the Docker image

### Verify UI Has Latest Code

Check if the UI includes the login page:

```bash
# Check if LoginPage exists in the built files
docker run --rm kubechronicle/ui:latest ls -la /usr/share/nginx/html/assets/ | grep -i login

# Or check the source
grep -r "LoginPage" ui/src/
```
