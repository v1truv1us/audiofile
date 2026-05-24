# CrateKeeper

Vinyl collection manager for serious diggers.

## Stack

- **Frontend**: Astro + Svelte 5 + Tailwind CSS
- **Backend**: Go + Chi router
- **Database**: Supabase Postgres (local dev via `supabase start`)
- **Auth**: Supabase Auth (passkeys + email/password)
- **Photos**: Supabase Storage
- **Search**: Postgres tsvector + GIN index

## Development

### Prerequisites

- [Supabase CLI](https://supabase.com/docs/guides/local-development)
- Go 1.26+
- Node 22+

### Database

```bash
supabase start   # Starts local Postgres + applies migrations
```

### Backend

```bash
cd backend
cp .env.example .env
go run ./cmd/server
```

Runs on `http://0.0.0.0:8080`. Health check: `GET /api/health`

### Frontend

```bash
cd frontend
npm install
npm run dev
```

Runs on `http://0.0.0.0:4321` (accessible on your LAN).

### E2E Tests

```bash
cd frontend
npm run test:e2e
```

## Architecture

See [docs/adr/](docs/adr/) for all architecture decisions.

## Project structure

```
cratekeeper/
├── backend/
│   ├── cmd/server/        # Entry point
│   ├── internal/
│   │   ├── collection/    # Collection handlers
│   │   └── wishlist/      # Wishlist handlers
│   └── go.mod
├── frontend/
│   ├── src/
│   │   ├── components/    # Svelte components
│   │   ├── layouts/       # Astro layouts
│   │   ├── lib/           # Shared utilities
│   │   ├── pages/         # Routes
│   │   └── styles/        # Global CSS
│   ├── e2e/               # Playwright tests
│   └── package.json
├── supabase/
│   └── migrations/        # Database migrations
└── docs/
    └── adr/               # Architecture Decision Records
```
