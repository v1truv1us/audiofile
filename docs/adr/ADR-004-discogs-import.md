# ADR-004: Discogs as Primary Import Source

## Status
Accepted

## Context
Vinyl collectors overwhelmingly use Discogs for cataloging. CrateKeeper should support importing collection data from Discogs to reduce manual entry friction.

## Decision
Support Discogs OAuth import flow in MVP+1. For MVP, allow manual entry with Discogs ID field for future matching. The releases table includes a discogs_id column for deduplication.

## Consequences
- MVP ships without Discogs OAuth (reduces scope)
- discogs_id enables future import without schema changes
- Users can still add records manually from day one
