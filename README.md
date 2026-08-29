# myNgrok

Self-hosted HTTP tunnel platform. The repository is being built in tracked phases; see
[`ngrok_clone_implementation_plan.md`](ngrok_clone_implementation_plan.md) for the
authoritative progress checklist.

## Development

```powershell
Set-Location backend; go run ./cmd/server
Set-Location agent; go run ./cmd/tunnel-agent version
```

When run directly, the backend listens on `HTTP_ADDR` (default `:8080`). Docker
Compose publishes it on host port `8095`.

Its initial health endpoint is:

```text
GET /health/live
GET /health/ready
```

Copy `.env.example` before configuring a real deployment. Secrets must not be
committed to the repository.

Docker Compose exposes its PostgreSQL container as `localhost:15432` (container
port `5432`), so it does not conflict with a PostgreSQL server already running
on the host. The backend service still connects to `postgres:5432` over the
internal Compose network.

## Containerized frontend

Build and start the complete local stack with:

```powershell
docker compose up --build
```

The Vue application is compiled in a Node build stage and served by nginx at
`http://localhost:5173`. nginx provides SPA route fallback and proxies `/api/`
requests to the internal backend service, so browser API calls stay on the same
origin. The root `.dockerignore` keeps development dependencies and generated
artifacts out of the Docker build context.

## Run the tunnel agent from Windows

The agent is a normal Windows executable; it does not require WSL, Docker, or
systemd. Start the local gateway with `docker compose up --build`, create an
account and an agent token at `http://localhost:5173/app/tokens`, then build and
run the agent:

```powershell
.\scripts\build-agent-release.ps1 -Version 0.1.0 -Target windows/amd64
.\dist\agent\tunnel-agent_0.1.0_windows_amd64.exe http 3000 `
  --token <AGENT_TOKEN> `
  --gateway ws://localhost:8095/api/v1/agent/connect
```

Replace `3000` with the port of the local service on the Windows computer. The
agent opens a persistent WebSocket, forwards HTTP requests to that service, and
reconnects automatically after a temporary gateway or network failure. Stop it
with `Ctrl+C`. For the full local and public-deployment walkthrough, see
[the Windows agent guide](docs/windows-agent.md). Release artifact details are
in [agent-releases.md](docs/agent-releases.md).

## Persistent data

Database migrations now prepare an `agents` table for agent identity, owning
user and token, connection status, and last-seen time. On every successful
WebSocket handshake the gateway creates or refreshes that record; every
heartbeat refreshes `last_seen_at`, and a closed socket marks the agent offline.

Signed-in users can fetch only their own agents through:

```text
GET /api/v1/agents
Authorization: Bearer <ACCESS_TOKEN>
```

The web app exposes the same data at `/app/agents`. It displays the hostname,
platform, version, last-seen time and a clear online/offline status. It does not
yet create public tunnels; that starts in Epic 6.

## Tunnel persistence

The first Epic 6 migration introduces the `tunnels` table. A tunnel is owned by
a user and an agent, has a unique subdomain, a local destination and an open or
closed lifecycle state. Tunnel opening and request forwarding are not available
until the following Epic 6 tasks are completed.

New tunnel addresses will use a cryptographically random, ten-character,
lowercase alphanumeric subdomain. This format is safe as a DNS label; the
database remains the final authority that prevents duplicates.

The gateway-to-agent protocol now reserves `open_tunnel` with `requestId` and
`localAddress` fields. The next tasks will handle this message, persist a tunnel
and return its public address.

Active tunnels are also kept in a thread-safe in-memory registry keyed by
subdomain. Closing an agent session removes every route owned by that session,
so disconnected agents cannot keep receiving public traffic.

When the gateway opens a tunnel it responds with `tunnel_opened`. The response
contains the original `requestId`, the persistent `tunnelId`, its `subdomain`,
and `publicUrl`; this lets an agent correlate the result with its request.

An agent may request removal with `close_tunnel`, supplying the same correlation
`requestId` and the `tunnelId`. The gateway will remove its active route and
persist the closed state when command handling is wired in the next lifecycle
step.

Signed-in users can list their own persisted tunnels with:

```text
GET /api/v1/tunnels
Authorization: Bearer <ACCESS_TOKEN>
```

Each result includes its ID, agent, subdomain, local address and lifecycle
status. The endpoint deliberately returns an empty list until gateway command
handling creates tunnels.

The web app exposes this list at `/app/tunnels`, including an active-tunnel
counter, status badges and a responsive empty state. It is read-only in this
phase; tunnel creation is driven by the agent protocol in the next phase.

## Public-host routing

Epic 7 resolves a tunnel from the request `Host` header in the form
`<subdomain>.tunnel.example.test` (configured via `PUBLIC_BASE_DOMAIN`). The
parser accepts a single DNS-safe subdomain label, is case-insensitive, and
ignores an optional port. Bare domains and nested labels are not tunnel routes.

The backend now uses this parser for all public fallback requests. Unknown
subdomains return `404`; an active registered route reaches the public handler
and currently returns `501` until request dispatch is added.

The first dispatch step now serializes one `http_request` message with a random
request ID, HTTP method and request path into the owning agent session. The
public handler waits for the response implementation; therefore it currently
returns `504` after successful queueing.

The agent now receives this message and exposes its request ID, method and path
to its local request handler. The next task will connect that handler to the
configured local HTTP destination.

The agent can now perform the first local-proxy slice: one buffered `GET` to
the local address supplied to `tunnel-agent http`. It supports response bodies
up to 1 MiB, preserves request and response headers (including repeated header
values), forwards a buffered request body, preserves local HTTP status, query
strings, and common HTTP methods. Streaming follows in later epics.

Before forwarding, the gateway strips standard hop-by-hop HTTP headers and headers
named by `Connection`; the agent applies the same rule at the local boundary.
The local service receives trusted `X-Forwarded-For`, `X-Forwarded-Host`,
`X-Forwarded-Proto`, and `X-Tunnel-Request-ID` fields generated by the gateway.

Every public request is assigned an opaque request ID. The gateway correlates its
pending response by that ID; this is the basis for concurrent tunnel forwarding.
The agent runs local requests concurrently and serializes WebSocket writes through a
single writer loop. `MaxConcurrentRequests` defaults to 16 per agent connection.

Pending public requests are bound to their owning agent session. If that WebSocket
connection closes, the gateway immediately releases those requests with `502 Bad
Gateway` instead of waiting for the normal tunnel response timeout.

The same disconnect lifecycle removes every in-memory route for that session and
marks its open persisted tunnels as closed. A stale subdomain therefore returns
`404` until the agent reconnects and reopens the tunnel.

After every successful reconnect the agent sends `open_tunnel` again for its
configured local address. The gateway restores the most recently closed tunnel
for that agent and destination, preserving its public subdomain, then returns a
fresh `tunnel_opened` message bound to the new session.

Both processes support graceful shutdown. Sending `Ctrl+C` or `SIGTERM` to the
backend closes active agent sessions before HTTP shutdown; the agent closes its
own WebSocket session and stops reconnecting.

Authentication endpoints for registration, login and refresh are rate-limited
to ten attempts per minute per client IP. Excess attempts receive `429 Too Many
Requests`, a `RATE_LIMITED` API error and a `Retry-After` response header.

Public tunnel ingress is separately limited to 120 requests per minute for each
combination of tunnel and client IP. This prevents one client from exhausting a
single agent tunnel while leaving other tunnels and clients unaffected.

Public tunnel request and response bodies are capped at 32 MiB. Oversized public
uploads receive `413 Request Entity Too Large`; oversized agent responses are
rejected by the gateway before they can exhaust its memory.

Tunnel protocol binary frames accept at most 32 KiB of payload on both the
gateway and agent. WebSocket messages are additionally capped at 64 KiB.

In HTTPS environments refresh tokens use an `__Host-refresh_token` cookie with
`Secure`, `HttpOnly`, `SameSite=Strict`, `Path=/` and no `Domain` attribute.
Development uses a non-secure cookie name so local HTTP testing continues to
work.

Browser API access is restricted by `CORS_ALLOWED_ORIGINS`, a comma-separated
origin allowlist. Development defaults to `http://localhost:5173` and
`http://127.0.0.1:5173`; production should explicitly set trusted HTTPS origins.

Control-plane API and health responses add `nosniff`, anti-clickjacking,
referrer, permissions, CSP and no-store headers. These are deliberately not
added to public tunnel responses, which belong to the proxied local application.

Revoking an agent token immediately disconnects every active WebSocket session
authenticated by that credential. Its pending requests and runtime tunnel routes
are then cleaned up through the normal disconnect lifecycle.

The backend emits JSON structured access logs with method, path, client IP and
request duration. Request bodies, authorization headers and credentials are not
logged.

Every HTTP response also includes an `X-Request-ID`. A valid client-provided
value is preserved; invalid values are replaced with a generated opaque ID,
which is included in the corresponding access-log entry.

`GET /metrics` exposes Prometheus-compatible HTTP request count, duration and
in-flight metrics. Labels are limited to HTTP method and route group (`api`,
`health`, `tunnel`) to avoid leaking IDs or causing high-cardinality metrics.
It also includes gateway agent-session gauges and connection/disconnection
counters.

The agent tracks its active session plus successful connection and disconnection
counters. It prints the final counts after a graceful stop; embedding callers
can receive updates through the gateway client session-metrics callback.

Gateway metrics also aggregate completed tunnel requests and total forwarded
request/response body bytes. They do not label individual tunnel IDs or
subdomains.

The backend container uses a multi-stage Go build and runs the resulting static
binary in a distroless non-root image (UID 65532).

The complete Epic 7 vertical slice now returns that buffered local status and
body to the public HTTP caller through the same WebSocket session. It is limited
to one GET request at a time; non-GET methods, request bodies and streaming are
deliberately deferred to Epic 8 and later.
