# Supabase Migration Handoff

**Date:** 2026-10-17 (initial work) · 2026-10-23 (handoff written)
**Status:** Ready for execution — waiting on database dumps from the developer's Mac
**Context:** AudioFile currently runs on a hosted free-tier Supabase project (`bwzldaesynlruqukbnej`, aws-1-us-east-2). Recurring free-tier pausing forces manual unpauses. A self-hosted Supabase stack is now running on our own infrastructure; this document captures the decision, the state of both sides, and the remaining steps.

---

## 1. Current state

### Hosted (production today)
- Project ref: `bwzldaesynlruqukbnej` (aws-1-us-east-2)
- App connects via `DATABASE_URL` (pooler, `postgres.bwzlda...`) — **working**
- `PUBLIC_SUPABASE_ANON_KEY` in Coolify is **rejected by the hosted project** ("Invalid API key"). JWT is structurally valid for the right ref, so the key was almost certainly **rotated** on the hosted side. Frontend Supabase calls (auth/storage from the browser) are likely silently failing. → Check dashboard → Settings → API and update Coolify env + rebuild.
- **Keepalive installed (2026-10-17):** Coolify host runs a daily `select 1` via `psql` (dockerized postgres:16-alpine) at 06:17 against the hosted DB, preventing free-tier auto-pause.
  - Script: `/usr/local/sbin/supabase-keepalive.sh` on `root@100.127.65.2` (Tailscale name for the Coolify host)
  - Failure log: `/var/log/supabase-keepalive.log`
  - Contains the DB URL — root-readable only (0700)

### Self-hosted (staging/destination)
- Coolify **service** `supabase` on the **v1truv1us dev server** (`31.97.215.183`, Coolify server name "v1truv1us dev server")
- Service UUID: `vlgjddu3mkgotyd2elr17bvk`; all 15 containers healthy as of 2026-10-17
- API endpoint / Kong: `https://supabase.v1truv1us.dev` (was the auto-generated UUID subdomain; changed 2026-10-17). Returns 401 without credentials — expected.
- Studio dashboard behind basic auth (creds in service env: `SERVICE_USER_ADMIN` / `SERVICE_PASSWORD_ADMIN`)
- DB: `supabase-db` container, image `supabase/postgres:15.8.1.085`, data in named volume

### Fix applied to the Coolify Supabase template (important context)
The one-click Supabase install was broken: the `minio-createbucket` init container referenced `minio/mc`, and **MinIO removed their images from Docker Hub** (registry returns a misleading "may require docker login" error). Fixed by patching the service's stored compose to use `pgsty/silo:RELEASE.2026-10-15T00-00-00Z` (same fix upstream Coolify merged 2026-10-15, commit `d60d3a5b`, issue coollabsio/coolify#11855). **Not in any Coolify release as of v4.4.6** — new Supabase one-click installs on this instance will fail until Coolify is upgraded past the release that ships `d60d3a5b`. Existing service is patched in-place in the DB (non-standard until the template is refreshed from the UI).

## 2. Decision

**Hybrid, migrate when ready:**
1. Production stays on hosted Supabase until the migration below is executed — do **not** cut over under pressure. The keepalive removes the pausing pain meanwhile.
2. The current self-hosted stack (`supabase.v1truv1us.dev`) is the **admin/playground instance**.
3. AudioFile gets a **dedicated second stack** (e.g. Coolify service `audiofile-supabase` → `audiofile-supabase.v1truv1us.dev`) so the app's production data is isolated. The Coolify Supabase template is single-project.
4. Before any data lands: configure scheduled backups (Coolify scheduled task, nightly `pg_dump` inside `supabase-db` + offsite copy) — the dev server has no backup story today.
5. Cutover only after verification; hosted project retained (paused is fine — keepalive can be disabled then) until confidence is established.

## 3. Remaining steps

### Developer (Mac)
```bash
brew install supabase/tap/supabase
supabase login

mkdir -p ~/Desktop/audiofile-migration && cd ~/Desktop/audiofile-migration
supabase link --project-ref bwzldaesynlruqukbnej

supabase db dump --file roles.sql --role-only
supabase db dump --file schema.sql
supabase db dump --file data.sql --data-only
```
`link` prompts for the DB password (resettable at dashboard → Settings → Database if unknown).

Also note, from the dashboard:
- **Storage** → bucket count / rough object count (if empty, migration is DB-only)
- **Authentication → Providers** → which providers are enabled. OAuth client secrets cannot be exported and must be re-entered on the self-hosted stack.

Hand the three `.sql` files to the operator/agent machine.

### Operator/agent (Coolify host `root@100.127.65.2`, this session's setup already done)
1. Inventory the dumps (tables, extensions, RLS, `auth.users` count).
2. Create Coolify service `audiofile-supabase` from the Supabase template — it will hit the `minio/mc` bug on Coolify ≤ 4.4.6; apply the same `pgsty/silo` patch (replace `image: minio/mc` in `docker_compose_raw`/`docker_compose` and the `minio-createbucket` ServiceApplication row).
3. Configure nightly backups of the new stack's `supabase-db` before restoring anything.
4. Restore in order: `roles.sql` (role-only) → `schema.sql` → `data.sql`, via `docker exec` into the new `supabase-db` container (SSH hop: coolify host → `ssh -i /tmp/devsrv_key coolify@31.97.215.183`; key material lives inside the coolify container, re-fetchable via tinker from `Server->privateKey` for UUID `d25e46o584pw8d3jqwxuncxe`).
5. Storage objects (if any): sync via S3 credentials from the new stack's minio (`AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY` = `SERVICE_USER_MINIO`/`SERVICE_PASSWORD_MINIO` in service env).
6. Rewire the audiofile Coolify app (`a7enf427mqokx7il22uibvmo`): `DATABASE_URL`, `SUPABASE_URL`, `PUBLIC_SUPABASE_ANON_KEY` (+ any service-role key env) to the new stack's values; force-rebuild so the client bundle picks up new keys. Health check is disabled on this app (distroless image, no curl/wget) — verify via `https://audiofile.app/api/health`.
7. Verify, then decommission: stop keepalive cron on the old host, keep hosted project as rollback.

## 4. Gotchas learned this session

- **AudioFile's Go binary exits when the DB ping fails.** A Supabase-side blip (pooler circuit breaker, 2026-10-17) caused a crash-loop and a 502 on audiofile.app until retries succeeded. Self-hosting moves this risk to our own infra — the dedicated stack + backups are the mitigation. Consider retry/backoff tuning as a separate task.
- The post-deployment command on the Coolify app fails harmlessly (`exec: "sh": executable file not found` — distroless). Cosmetic; ignore or remove.
- Coolify app `audiofile` has **no container health check** (distroless, no shell). Monitoring gap: rely on `audiofile.app/api/health` external checks or a Go static health binary in the Dockerfile.
- `PUBLIC_SENTRY_DSN`/`SENTRY_DSN` env vars were set on the Coolify app (2026-10-17); the Go backend logs "Sentry initialized". Frontend DSN status unverified (see rotated-key issue above).
- Self-hosted stack shares the dev server with ~20 other containers. Fine for staging; revisit placement if it becomes production (disk, backup cadence, blast radius).

## 5. Key references

| Thing | Where |
|---|---|
| Coolify host | `ssh root@100.127.65.2` (Tailscale) |
| Dev server (runs Supabase) | `31.97.215.183`, user `coolify`; reachable via coolify container SSH key or Coolify UI terminal |
| Supabase service (admin) | Coolify service UUID `vlgjddu3mkgotyd2elr17bvk` → `https://supabase.v1truv1us.dev` |
| AudioFile Coolify app | UUID `a7enf427mqokx7il22uibvmo`, repo `v1truv1us/audiofile`, deploys `main` |
| Keepalive | `/usr/local/sbin/supabase-keepalive.sh`, cron 06:17 daily, on Coolify host |
| Upstream fix tracking | coollabsio/coolify#11855, commit `d60d3a5b` — upgrade Coolify when a release ships it |
