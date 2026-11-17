# Authentication and credential contract

The CLI keeps three credentials together: the Qobuz App ID, its App Secret, and
the user token issued while using that App ID. A saved token records its issuing
App ID. Replacing the App ID invalidates the saved token unless the user
explicitly supplies `--app-id` to pin the pair.

## Implementation

`internal/auth.Manager` owns the credential and login sequence. The CLI is a
thin adapter: `setupClient` maps flags to `auth.Options`, calls `Prepare()`, and
uses the returned API client and account. `auth doctor` uses
`PrepareDiagnostics()` and `ValidateUserToken()`, which prepare and validate
credentials without logging in or writing `account.json`.

`Manager` exposes its furthest successful stage:

| State | Meaning |
| --- | --- |
| `new` | No account or credential work has started. |
| `account_loaded` | The persisted account was loaded (or initialized empty). |
| `app_credentials_ready` | An App ID/Secret pair exists and an API client was created. |
| `secret_valid` | Secret validation classified the pair as valid. |
| `token_valid` | An existing token passed `user/login` validation. |
| `authenticated` | A fresh password login succeeded. |
| `unauthenticated` | The server started without a user token. |

Tests inject account storage, secret discovery, client construction, and sleep
through `auth.Dependencies`, so the state machine is verified without network
or filesystem side effects.

## When checks run

- `dl` and `search` initialize app credentials, validate the selected secret,
  validate an existing token, and log in only when necessary.
- `serve` performs the same app credential checks but may start without a user
  token. Its current public endpoint behavior is unchanged.
- Explicit `--email` always starts a fresh password login, even for the same
  email address. `--email` and `--token` are mutually exclusive.
- Resource metadata requests report missing tracks/albums independently from
  authentication failures.

## Secret validation

Secret validation sends a signed `track/getFileUrl` request for a known track.
It validates the signature, not the user's subscription or the existence of the
test track.

| Result | Classification | Action |
| --- | --- | --- |
| 200 | Valid secret | Continue. |
| 400 with `Invalid Request Signature` | Invalid secret | Refresh bundle credentials and retry once. |
| 401 | Valid secret, missing/expired user token | Continue to token validation; do not refresh the secret. |
| 401 with an App ID rejection message | Inconclusive App ID failure | Report the mismatch; do not rotate the secret. |
| 403 | Valid secret, account/region/entitlement restriction | Continue to token validation; do not refresh the secret. |
| 404 | Inconclusive | Retry validation once; never rotate credentials for a missing test track. |
| 429 | Inconclusive | Retry once after a short delay; do not rotate credentials. |
| 5xx or transport failure | Inconclusive | Retry once; report the connection/API problem if it persists. |
| Other 400 | Inconclusive | Do not assume the signature is invalid. |

## Token and login validation

Token validation calls `user/login` with `user_auth_token` and the current App
ID. A successful response must be JSON with a non-empty `user_auth_token`.

| Result | Classification | Action |
| --- | --- | --- |
| 200 with non-empty token | Authenticated | Store the token and its issuing App ID. |
| 200 HTML/non-JSON or empty token | Invalid response | Report a proxy/upstream response problem; do not rotate secrets. |
| 401 token/authentication error | Invalid token | Clear the token and use a saved password or interactive login. |
| 401 password login | Bad credentials | Report incorrect email/password; do not retry through another host. |
| 400/401 App ID rejection | App ID mismatch | Retry once with the saved issuing App ID when available. |
| 403 | Forbidden | Report the account/region restriction; do not refresh credentials. |
| 429 | Rate limited | Do not retry the password through another host. |
| 5xx or transport failure | Temporary failure | A proxy login may retry once against the direct API. |

Only transport and 5xx login failures use request-scoped direct fallback. This
fallback must not permanently change the client's configured API base URL.

## Failure behavior

- Invalid explicitly supplied app credentials produce an error; they are never
  silently replaced by automatically discovered credentials.
- 401, 403, and 429 stop quality fallback. They are not quality availability
  responses.
- `auth doctor` reports the same classifications without printing passwords,
  complete tokens, or app secrets. It is read-only and never submits a password
  or rewrites `account.json`.
- Account storage uses `obf:v1:` XOR/Base64 obfuscation for local sensitive
  fields. This is not cryptographic protection against local forensic access.
