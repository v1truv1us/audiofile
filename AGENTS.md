# AudioFile agent instructions

These rules apply to this repository.

## Test-driven development is required

- Write or update tests before changing behavior. For bug fixes, add a regression test that fails for the reported behavior before changing production code.
- Do not treat a feature as complete until the relevant automated tests pass.
- Keep coverage high for backend services: `backend/internal/...` must stay at or above 90% statement coverage.
- Prefer tests for cases that are easy to miss manually:
  - upstream API timeouts, bad status codes, malformed JSON, and fallback behavior
  - database errors, not-found paths, failed commits/rollbacks, and transaction boundaries
  - validation and authorization/user-scope requirements
  - empty results, nil/nullable DB values, cache hits, and malformed input
- Avoid shallow tests that only mirror implementation details. Test observable behavior at the handler/component boundary when practical.

## Documentation and decision records

- Update documentation in the same change when behavior, setup, public commands, environment variables, deployment assumptions, or user-facing workflows change.
- Add or update an ADR under `docs/adr/` when making an architectural decision, including provider choices, auth/data ownership changes, persistence model changes, external service integrations, deployment/domain decisions, or irreversible tradeoffs.
- ADRs should capture the decision, context, consequences, and alternatives considered. Keep them short, but make the why recoverable later.
- Do not leave decisions only in chat. If a future maintainer would ask “why is it this way?”, write it down.
- If no documentation update is needed, state that explicitly in the handoff and why.

## Verification commands

Backend service coverage gate:

```bash
./scripts/check-backend-coverage.sh
```

Full backend compile/test pass:

```bash
cd backend && go test ./...
```

Frontend build verification:

```bash
cd frontend && bun run build
```

If a requested change cannot be covered with a useful automated test, state why before implementation and use the closest executable check available.
