# KubeChronicle UI

A React-based frontend for exploring Kubernetes resource change history.

## Technology Stack
- **Framework**: React 18+ (via Vite)
- **Language**: TypeScript
- **Styling**: Tailwind CSS (v4)
- **Routing**: React Router v6+
- **HTTP Client**: Axios
- **Icons**: Lucide React
- **Date Formatting**: date-fns

## Project Structure
```
ui/
├── src/
│   ├── api/          # API client and endpoints
│   ├── components/   # Reusable UI components
│   │   ├── DiffViewer/ # JSON Patch visualizer
│   │   ├── Timeline/   # Change event list
│   │   ├── Filters/    # Search filters
│   │   └── Layout.tsx  # Main app layout
│   ├── pages/        # Route components (Timeline, Detail, Resource, Actor)
│   ├── types/        # TypeScript definitions
│   ├── routes/       # App routing configuration
│   └── utils/        # Helper functions
```

## Setup & Running

### Prerequisites
- Node.js (v18+)
- Backend API running (default: http://localhost:8080)

### Development
1. Install dependencies:
   ```bash
   cd ui
   npm install
   ```
2. Start development server:
   ```bash
   npm run dev
   ```
   The UI will be available at http://localhost:5173.

### Build
To build for production:
```bash
npm run build
```
Artifacts will be in `dist/`.

## API Configuration
Refers to endpoints defined in `../docs/api.md`.
Base URL defaults to `http://localhost:8080`.
To configure, use `.env` file:
```
VITE_API_URL=http://your-api-url
```

## Features
- **Global Timeline**: View all changes across the cluster.
- **Resource History**: Filter changes by specific resource.
- **Change Detail**: detailed view including JSON Patch diff visualization.
- **Actor Activity**: Trace changes made by a specific user.
- **Filtering**: Filter by namespace, kind, name, user, operation.

## Extending
- **Add new pages**: Create component in `src/pages` and add route in `src/routes/index.tsx`.
- **Add new components**: Place in `src/components`.
- **Styling**: Use tailwind classes. Global styles in `src/index.css`.
