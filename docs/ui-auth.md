# UI Authentication

The kubechronicle UI includes a complete authentication system with login page, protected routes, and user session management.

## Features

- **Login Page**: Clean, user-friendly login interface
- **Protected Routes**: Automatic redirect to login when authentication is required
- **Session Management**: JWT tokens stored in localStorage
- **Auto Token Injection**: API requests automatically include authentication tokens
- **User Display**: Shows logged-in user and roles in the header
- **Logout**: Easy logout functionality
- **Admin Detection**: Automatically shows/hides admin features based on user role
- **Auth Detection**: Automatically detects if authentication is enabled on the backend

## Components

### AuthContext (`src/contexts/AuthContext.tsx`)

Provides authentication state and methods:
- `user`: Current user object (username, roles, email)
- `token`: JWT token
- `login(username, password)`: Login function
- `logout()`: Logout function
- `isAuthenticated`: Boolean indicating if user is logged in
- `isAdmin`: Boolean indicating if user has admin role
- `loading`: Boolean indicating if auth state is being loaded

### LoginPage (`src/pages/LoginPage.tsx`)

Login form with:
- Username and password fields
- Error message display
- Loading state during login
- Automatic redirect after successful login

### ProtectedRoute (`src/components/ProtectedRoute.tsx`)

Route wrapper that:
- Checks if authentication is enabled on backend
- Redirects to login if auth is required and user is not authenticated
- Checks admin role for admin-only routes
- Shows access denied message for insufficient permissions

### Updated API Client (`src/api/client.ts`)

- Automatically includes `Authorization: Bearer <token>` header in all requests
- Handles 401 responses by clearing auth and redirecting to login
- Works seamlessly with authentication

## Usage

### Login Flow

1. User navigates to any protected route
2. If not authenticated, redirected to `/login`
3. User enters credentials
4. On success, token stored in localStorage
5. User redirected to original destination

### Logout Flow

1. User clicks logout button in header
2. Token and user data cleared from localStorage
3. User redirected to login page

### Admin Features

- Patterns page is only visible to admins
- Admin-only routes show access denied for non-admin users
- User roles displayed in header

## Authentication Detection

The UI automatically detects if authentication is enabled:

1. **Auth Enabled**: Requires login, shows login page, protects routes
2. **Auth Disabled**: Allows access to all routes without login

Detection works by checking if API endpoints return 401 Unauthorized.

## API Integration

### Login Endpoint

```typescript
POST /api/auth/login
Content-Type: application/json

{
  "username": "admin",
  "password": "password"
}

Response:
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "username": "admin",
    "roles": ["admin", "viewer"],
    "email": "admin@example.com"
  }
}
```

### Token Usage

All API requests automatically include:
```
Authorization: Bearer <token>
```

## Storage

- **Token**: Stored in `localStorage` as `auth_token`
- **User**: Stored in `localStorage` as `auth_user` (JSON string)

## Security Considerations

- Tokens stored in localStorage (consider httpOnly cookies for production)
- Tokens automatically included in all API requests
- 401 responses trigger automatic logout
- Admin routes protected on both frontend and backend

## Customization

### Styling

The login page uses Tailwind CSS classes and can be customized by modifying `LoginPage.tsx`.

### Redirect After Login

The login page redirects to the originally requested page (stored in `location.state.from`), or to `/` by default.

### Error Messages

Error messages are displayed in the login form and can be customized in the `handleSubmit` function.
