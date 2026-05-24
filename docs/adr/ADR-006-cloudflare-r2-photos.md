# ADR-006: Cloudflare R2 for Photo Storage

## Status
Accepted

## Context
User-uploaded photos of records (sleeve, label, vinyl condition) need object storage. We want S3-compatible APIs, zero egress fees, and a reasonable free tier.

## Decision
Use Cloudflare R2 for all photo storage. Store r2_key and r2_thumbnail_key in the item_photos table. Generate pre-signed URLs for uploads on the backend. Serve photos via R2 public bucket or Workers CDN.

## Consequences
- Zero egress cost (unlike S3)
- S3-compatible SDK works without vendor lock-in
- Requires Cloudflare account
- Thumbnail generation needed on upload (planned: sharp via Worker or Go resize)
