# ADR-003: Separate Global Release Metadata from User Ownership

## Status
Accepted

## Context
Multiple users may own the same release (e.g., a specific pressing of Dark Side of the Moon). Storing full release metadata per-user wastes space and creates inconsistency.

## Decision
Introduce a global `releases` table containing immutable metadata (title, artist, year, label, catalog_no, format, cover_url). A separate `collection_items` table links users to releases with user-specific data (condition, purchase price, notes). Releases are populated on first import and shared across users.

## Consequences
- No duplicate release metadata
- Consistent cover art and metadata across users
- Release edits (e.g., cover URL update) benefit all users
- Requires join query for collection views
