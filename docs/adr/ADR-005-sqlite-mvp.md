# ADR-005: SQLite for MVP, Postgres Migration Path

## Status
Accepted

## Context
AudioFile is initially a single-user or small-household app. A full Postgres setup adds operational overhead that isn't justified at MVP scale.

## Decision
Use SQLite (via go-sqlite3) for MVP with WAL mode and foreign keys enabled. Schema and queries will be written to be compatible with Postgres for future migration. No SQLite-specific functions will be used in business logic queries.

## Consequences
- Zero-config local database for MVP
- WAL mode supports concurrent reads during writes
- Migration to Postgres is possible without query rewrites
- FTS5 (SQLite-specific) will require pg_trgm or Meilisearch equivalent on Postgres migration
