# `internal/auth` — custom Go authentication

A self-contained Go package that handles Google OAuth (OIDC) and Twilio Verify SMS sign-in, issues HMAC-signed session cookies, and exposes a Gin middleware that resolves the cookie to a `userAccountID`. Replaces the embedded Better Auth Node sidecar for the new flow; coexists with it during cutover.

## Why this exists

The Better Auth sidecar was operationally painful (separate Node process, bash supervisor, npm-registry boot dependency, $7/mo always-warm machine). For a Google-OAuth + SMS-OTP feature set, those costs aren't justified. This package replaces it with a Go-native implementation that:

- Lives in the same binary as the API (no sidecar, no `start.sh`)
- Delegates every security-critical primitive to a vetted library (see "What we delegate" below)
- Is small enough for a single human to read end-to-end in 30 minutes

It is deliberately NOT a general-purpose auth framework. It does Google + SMS, period. Adding a new identity source means writing more handlers; that's intentional.

## What's in the box

| Route | Verb | Purpose |
|---|---|---|
| `/auth/google/start` | GET | Begin OAuth flow. Generates state+nonce+PKCE, sets state cookie, 302s to Google. |
| `/auth/google/callback` | GET | Verify state cookie, exchange code, verify ID token, find/create user, set session cookie, 302 to FE. |
| `/auth/sms/send` | POST | Trigger Twilio Verify OTP. Always 204; never reveals whether the phone is registered. |
| `/auth/sms/verify` | POST | Validate OTP via Twilio. On approval, find/create user, set session cookie. |
| `/auth/sign-out` | POST | Delete session row, clear cookie. |
| `/auth/session` | GET | Return `{user: {...}}` or `{user: null}` for FE bootstrap. |

The package's only contract with the rest of the API is `auth.CurrentUser(c) (uuid.UUID, bool)`. Handlers don't know cookies exist.

## What we delegate (and why)

| Concern | Library | Why |
|---|---|---|
| OAuth code+PKCE flow | `golang.org/x/oauth2` | Google's own library; handles state, code exchange, token refresh, PKCE challenge. |
| OIDC ID-token verification | `github.com/coreos/go-oidc/v3` | Used by Kubernetes, Tailscale, Red Hat. Verifies signature, audience, issuer, expiry, nbf with clock skew. |
| OTP generation/storage/validation | Twilio Verify (REST) | Twilio handles the OTP lifecycle and fraud protection. We never see the code. |
| Random session IDs / state / nonce | `crypto/rand` | Standard library, OS-backed CSPRNG. |
| HMAC + constant-time compare | `crypto/hmac`, `crypto/subtle` | Standard library. |
| Per-bucket rate limiting | `golang.org/x/time/rate` | Standard extended library, used everywhere. |

What we explicitly DO NOT do:

- Hand-roll any cryptographic primitives.
- Implement OIDC discovery or JWS signature verification ourselves.
- Generate or store SMS OTP codes.
- Build a session "cache" — every authenticated request hits Postgres for the session row. The cost is one indexed lookup; the benefit is instant revocation via row delete.

## Threat model

Each row pairs a defense with the test that proves it. Tests are not in this PR yet (deferred to a follow-up); the table is the spec for what those tests must assert.

| Threat | Defense | Test name (todo) |
|---|---|---|
| Cookie tampering | HMAC-SHA256 of session ID; `subtle.ConstantTimeCompare` on read. | `TestCookie_Tampered_Rejected` |
| Session fixation | Fresh 32-byte `crypto/rand` ID on every login (Google callback or SMS verify). Pre-login cookie value can never become a post-login session ID. | `TestLogin_NewSessionID` |
| Session expiry not enforced | DB `expires_at > NOW()` checked in `repository.AuthSessionRepository.Get`; expired rows return `ErrSessionNotFound`. | `TestExpiredSession_Rejected` |
| Session lasts forever despite sliding TTL | Absolute cap: `now - created_at >= SessionAbsoluteMaxAge` (default 90d) → row deleted, cookie cleared, force re-auth. Sliding TTL bumps `expires_at`, never `created_at`. | `TestAbsoluteMaxAge_ForcesReauth` |
| Logout doesn't actually log out | `/auth/sign-out` deletes the row AND `Max-Age=0`'s the cookie. Same value can't re-authenticate. | `TestSignOut_CookieReusedAfter_Rejected` |
| Cookie attribute downgrade (Secure/HttpOnly/SameSite/Domain) | `__Host-` cookie name prefix forces all four (browser refuses violation); single `setSessionCookie` writer is the only emitter. | `TestSessionCookie_Attributes` (literal-string assert) |
| OAuth state CSRF | HMAC-signed state cookie holds (state, nonce, PKCE verifier, expires); callback verifies cookie HMAC, then `subtle.ConstantTimeCompare(queryState, cookieState)`. | `TestGoogleCallback_NoStateCookie_Rejected`, `TestGoogleCallback_MismatchedState_Rejected` |
| State cookie reuse | Cleared at the **start** of `handleGoogleCallback`, before validation, so even an attacker reaching the callback first invalidates the legitimate user's pending state. | `TestStateCookie_OneTimeUse` |
| OAuth redirect injection / open redirect | Callback always 302s to `cfg.FrontendBaseURL`. We never read `redirect_uri` / `next` / `return_to` from query params. | `TestGoogleCallback_RedirectIsFrontendBaseURL` |
| ID token replay / forged token | `coreos/go-oidc` verifies signature against Google's JWKS, plus `aud`, `iss`, `exp`, `nbf` with default ±30s clock skew. We additionally check `nonce` matches the value we put in the state cookie. | `TestIDToken_BadIssuer/BadAudience/Expired/SkewedClock` |
| Unverified email accepted as identity | `email` is only set on the user row when Google's `email_verified` claim is true. Stable identity is `sub`, never email. | `TestGoogle_UnverifiedEmail_NotPersisted` |
| Account collision via recycled email | Identity lookup is `INSERT ... ON CONFLICT (provider, provider_id) DO UPDATE` keyed on Google `sub`, NOT email. | `TestGoogle_NewSubGetsNewUser_EvenIfEmailReused` |
| Concurrent first-login race | Same `ON CONFLICT` upsert atomic; 50 simultaneous logins for one `sub` produce exactly one row. | `TestConcurrent_GetOrCreateByProviderIdentity` |
| CSRF on `/auth/sms/*`, `/auth/sign-out` | `requireOrigin` middleware: Origin header must be present and in the allowlist (matches CORS allowlist). | `TestSmsVerify_MissingOrigin_Rejected`, `..._BadOrigin_Rejected` |
| User enumeration via `/auth/sms/send` | Always returns 204. Validation, rate-limiting, and Twilio failures all collapse to the same response. Twilio rate-limits also collapse to 204 inside `sendVerification`. | `TestSmsSend_NoEnumerationLeak` |
| Twilio bill drained by attacker | Per-phone limiter (3 / 10min) AND per-IP limiter (10 / 10min). Both must pass; we never reveal which one tripped. Compensating control: Twilio Verify's own per-phone limits + fraud-detection settings. | `TestSmsSend_RateLimited_PerIP`, `TestSmsSend_RateLimited_PerPhone` |
| Cookie + Bearer ambiguity during cutover | `getGoogleAuthMiddleware` (legacy) skips its work when `userAccountID` is already set on the context. Cookie wins; same user_account_id flows through regardless. | `TestMiddleware_CookieWinsOverBearer` |
| Logging leaks sessions/OTPs/cookies | `logf` wraps `log.Printf` and is the only logger. Errors deliberately summarize ("ok"/"fail", "rate limited") rather than echo input. Twilio creds are passed via `SetBasicAuth` (never formatted into strings). | Code review |

## Defense layers we DO NOT own

These are real risks, but they live outside `internal/auth/`. Documenting them here so future-you doesn't think this package is the place to fix them.

| Threat | Where it's handled | Status today |
|---|---|---|
| XSS → cookie theft | FE code review + CSP header | Mitigated by `HttpOnly` cookies (theft impossible); fetch-from-victim's-browser still possible. CSP not yet set on CloudFront. |
| Subdomain takeover → cookie injection | Cookie is `__Host-` + host-only on `api.factor.trade`, NOT `.factor.trade`. Sibling subdomains can't write our cookie. | Mitigated by design. |
| HSTS missing → first-visit MITM | CloudFront + Fly response headers | TODO. ~5 min change at the edge. |
| SIM swap → SMS factor compromised | SMS is fundamentally a low-assurance factor. | Accepted risk for current threat model. Don't gate high-value actions behind SMS only. |
| Multi-instance rate-limit bypass | `internal/auth/ratelimit.go` uses an in-memory bucket, per process. | **Known gap.** Compensating: Twilio Verify's own per-phone limits + cost monitoring. Replace with a shared backend if abuse materializes. |
| Compromised admin credentials → cookie forgery | `SessionSecret` rotation invalidates all sessions. Runbook below. | Mitigated by rotation procedure. |

## Operational runbooks

### If `SessionSecret` leaks

1. Generate a new value: `openssl rand -hex 32`.
2. Update the Fly secret: `flyctl secrets set sessionSecret=<new>`.
3. Truncate the session table to invalidate any in-flight sessions: `DELETE FROM app_auth.user_session;`.
4. Deploy. All users re-authenticate.

### If `googleClientSecret` leaks

1. Rotate in Google Cloud Console (creates a new secret, invalidates the old).
2. Update Fly secret: `flyctl secrets set googleClientSecret=<new>`.
3. Deploy. In-flight Google sessions remain valid (we have the ID token claims already); only future sign-ins use the new secret.

### If `twilioAuthToken` leaks

1. Rotate in Twilio Console.
2. Update Fly secret: `flyctl secrets set twilioAuthToken=<new>`.
3. Deploy. In-flight sessions unaffected.

### Daily session cleanup

Wired in `cmd/api/main.go`: `apiHandler.AuthService.RunSessionCleanup(ctx, 24*time.Hour)`. Best-effort; transient failures are logged and retried on the next tick. If `app_auth.user_session` ever grows pathologically, that's where to look.

## Defaults that bake in security

These are set in `Config` and matter:

- `SessionTTL` = 30 days (sliding). Bumped on every authenticated request.
- `SessionAbsoluteMaxAge` = 90 days (hard cap). Never bumped. Forces re-auth past this even for active users.
- `SessionSecret` minimum length: 32 bytes. `New()` refuses to construct with less.
- Cookie name: `__Host-factor_session`. Browser-enforced contract: Secure, host-only, Path=/.
- State cookie TTL: 10 min. Long enough for slow connections, short enough to bound exposure.
- SMS rate limits: 3/phone/10min, 10/IP/10min. Both must pass.

## Future work (not in this PR)

1. Tests for every row in the threat-model table. Spec is the table; implementation is mechanical.
2. `gosec` static analysis in CI (catches a different bug class than tests).
3. HSTS header at CloudFront and Fly.
4. FE swap: replace `frontend/src/auth.tsx`'s `better-auth/react` calls with direct fetches against `/auth/*`.
5. Delete `auth-service/`, `scripts/start.sh`, `api/auth_proxy.go`, the Better Auth `auth` schema, and the Node stage of `Dockerfile.fly` once the FE has cut over and prod traffic on `/api/auth/*` has been zero for a few days.
6. Postgres-backed rate limiter if the in-memory gap becomes exploitable in practice.
7. Key-id (kid) on session cookie HMAC for seamless secret rotation without forced logout.

Each of these is independently shippable. Don't try to bundle.
