# CrateKeeper

Vinyl collection manager for serious diggers.

## Stack

- **Frontend**: Astro + Svelte 5 + Tailwind CSS
- **Backend**: Go + Chi router
- **Database**: SQLite (WAL mode) → Postgres migration path
- **Auth**: WebAuthn passkeys + email/password fallback
- **Photos**: Cloudflare R2
- **Search**: SQLite FTS5

## Development

### Backend

```bash
cd backend
go run ./cmd/server
```

Runs on `http://localhost:8080`. Health check: `GET /api/health`

### Frontend

```bash
cd frontend
npm install
npm run dev
```

Runs on `http://localhost:4321`.

## Architecture

See [docs/adr/](docs/adr/) for all architecture decisions.

## Project structure

```
record-keeper/
├── backend/
│   ├── cmd/server/        # Entry point
│   ├── internal/
│   │   ├── db/            # Schema, migrations
│   │   ├── auth/          # WebAuthn, JWT
│   │   ├── collection/    # Collection handlers
│   │   └── wishlist/      # Wishlist handlers
│   └── go.mod
├── frontend/
│   ├── src/
│   │   ├── layouts/       # Astro layouts
│   │   ├── pages/         # Routes
│   │   └── styles/        # Global CSS
│   └── package.json
└── docs/
    └── adr/               # Architecture Decision Records
```
