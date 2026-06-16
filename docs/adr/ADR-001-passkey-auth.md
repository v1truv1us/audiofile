# ADR-001: Passkey Authentication with Email/Password Fallback

## Status
Accepted

## Context
AudioFile needs user authentication. Passkeys (WebAuthn) offer phishing-resistant, password-free auth with biometric convenience. However, passkey support varies by device/browser, requiring a fallback for older clients.

## Decision
Implement WebAuthn passkeys as the primary authentication method, with email/password as fallback. Use the `go-webauthn/webauthn` library for the Go backend.

## Consequences
- Users on modern devices get a seamless passkey experience
- Email/password fallback ensures universal access
- Slightly more complex backend auth implementation
- No third-party auth service dependency (data stays self-hosted)
