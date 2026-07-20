# AudioFile

Vinyl collection manager for serious diggers.

## Stack

- **Frontend**: Astro + Svelte 5 + Tailwind CSS
- **Backend**: Go + Chi router
- **Database**: Supabase Postgres (local dev via `supabase start`)
- **Auth**: Supabase Auth (email/password)
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

### Notifications & sharing

Sharing a wishlist to a user creates an in-app notification (bell in the nav, inbox at `/notifications`) and sends a best-effort email via [Resend](https://resend.com). Logged-in users viewing a public share link (`/wishlist?share=...`) can add it to their "Shared with me" inbox with one click. Optional env vars: `SUPABASE_SERVICE_ROLE_KEY` (server-only, for recipient email lookup), `RESEND_API_KEY`, `RESEND_FROM_EMAIL`. See ADR-010.

## Project structure

```
audiofile/
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

## Deployment

Production runs on [Coolify](https://coolify.fergify.work) (`audiofile` app, `main` branch).

### Automatic deploy (GitHub Actions)

Pushes to `main` run `.github/workflows/deploy.yml`, which calls the Coolify deploy API.

Repository secrets (Settings → Secrets and variables → Actions):

| Secret | Value |
|--------|--------|
| `COOLIFY_BASE_URL` | `https://coolify.fergify.work` |
| `COOLIFY_API_TOKEN` | API token from Coolify → Keys & Tokens |
| `COOLIFY_APP_UUID` | `a7enf427mqokx7il22uibvmo` |

You can also deploy manually from the Coolify UI or locally:

```bash
export COOLIFY_API_TOKEN=your-token
./scripts/deploy-coolify.sh
```

### Optional: GitHub → Coolify webhook

In the Coolify app → Webhook tab, copy the **GitHub** manual webhook URL and add it under the repo’s GitHub **Settings → Webhooks** (push events, `application/json`). This deploys without Actions if you prefer webhooks only.

## Post-Deploy Smoke Tests

AudioFile uses an automated post-deploy smoke test pipeline:

1. **AudioFile** deploys via Coolify.
2. Coolify runs post-deployment command: `curl -fsS -X POST https://r114tdxpc3tiziic7ynxz5in.fergify.work/run`
3. **Smoke Runner** (`audiofile-smoke-runner`) executes Playwright tests against production.
4. **Browserless** (`audiofile-browserless`) provides the headless Chromium runtime.

### Smoke test coverage

- `/api/health` returns 200
- `/api/releases/search?q=...` returns recognizable results
- `/api/releases/scan` accepts barcode lookups
- `/login/` renders login page
- `/signup/` renders signup page
- `/collection/` redirects unauthenticated users to login

### Manual trigger

```bash
curl -X POST https://r114tdxpc3tiziic7ynxz5in.fergify.work/run
```

### Check status

```bash
curl https://r114tdxpc3tiziic7ynxz5in.fergify.work/health
```

