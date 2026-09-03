# Backend: техническая документация

## Назначение

`backend` — Go gateway для self-hosted HTTP tunnels. Он одновременно обслуживает
control plane (браузерный API, аутентификация, токены, список ресурсов) и data
plane (публичный HTTP request → WebSocket agent → локальный HTTP service).

```text
Browser ──HTTP──> /api/v1/* ────────────────> PostgreSQL
Internet ─HTTP─> <subdomain>.<base-domain> ─> gateway ─WebSocket─> agent ─HTTP─> localhost
```

Точка входа — `cmd/server/main.go`. Внутренние пакеты находятся под
`internal/`, поэтому они не являются публичным Go API для других модулей.

## Структура

| Путь | Ответственность |
|---|---|
| `cmd/server` | Сборка зависимостей, запуск HTTP server, graceful shutdown. |
| `internal/config` | Чтение и валидация environment configuration. |
| `internal/database` | `pgxpool`, ping и последовательное применение migrations. |
| `internal/auth` | Password hashing, JWT access token, refresh-session, HTTP auth API. |
| `internal/agenttokens` | Создание, хранение hash, отзыв agent credentials. |
| `internal/agents` | Persistent identity и connected state agent instances. |
| `internal/gateway` | WebSocket agent sessions, request correlation и frames. |
| `internal/tunnels` | Persistent tunnels, host routing, public proxy, traffic counters. |
| `internal/protocol` | Версия wire protocol и binary body frames. |
| `internal/server` | Router, middleware, logs, CORS, headers и Prometheus metrics. |
| `migrations` | Неизменяемые SQL schema migrations. |

## Запуск и жизненный цикл

1. `config.Load()` валидирует environment.
2. `database.Open()` создаёт `pgxpool.Pool`, сразу проверяет соединение.
3. `Pool.Migrate()` создаёт `schema_migrations` и применяет новые migration в
   транзакциях.
4. `main` создаёт сервисы, runtime `Registry`, `SessionManager` и middleware.
5. HTTP server принимает control-plane и public tunnel traffic.
6. При `SIGINT`/`SIGTERM` `SessionManager.CloseAll()` отменяет agent sessions,
   затем `http.Server.Shutdown()` ждёт активные HTTP requests до 10 секунд.

## Конфигурация (`internal/config`)

`Config` содержит `Environment`, `HTTPAddr`, `PublicBaseDomain`,
`DatabaseURL`, `LogLevel`, `CORSAllowedOrigins`, request/heartbeat timeouts и
вложенный `AuthConfig`.

Обязательные production значения: `DATABASE_URL`, `JWT_ACCESS_SECRET` и
`JWT_REFRESH_SECRET`. В development используются безопасные для локальной
работы defaults, но production secrets нельзя оставлять значениями из
`.env.example`.

| Переменная | Значение |
|---|---|
| `HTTP_ADDR` | Адрес backend внутри процесса, обычно `:8080`. |
| `PUBLIC_BASE_DOMAIN` | Базовый domain для Host routing, например `tunnel.example.com`. |
| `DATABASE_URL` | PostgreSQL connection string. |
| `CORS_ALLOWED_ORIGINS` | CSV allowlist browser origins. |
| `TUNNEL_REQUEST_TIMEOUT` | Максимальное ожидание ответа agent. |
| `JWT_*_SECRET`, `*_TOKEN_TTL` | Ключи и lifetime auth tokens. |

## Хранилище (`internal/database`, `migrations`)

`Pool` оборачивает `*pgxpool.Pool`:

| Метод | Поведение |
|---|---|
| `Open(ctx, url)` | Валидирует URL, создаёт pool и делает ping. При failed ping закрывает pool. |
| `Raw()` | Возвращает pgx pool для repository/service queries. |
| `Ping(ctx)` | Проверяет доступность PostgreSQL. |
| `Migrate(ctx)` | Применяет каждую отсутствующую migration ровно один раз. |
| `Close()` | Закрывает pool на shutdown. |

Основные таблицы:

| Таблица | Назначение |
|---|---|
| `users` | Email, bcrypt password hash, status, login timestamps. |
| `refresh_tokens` | Только hash refresh token, expiry, revoke metadata. |
| `agent_tokens` | Hash opaque `tkn_` token, visible prefix, lifecycle. |
| `agents` | Инстанс agent, platform/version, `connected`, `last_seen_at`. |
| `tunnels` | Owner, agent, subdomain, local address, open/closed lifecycle. |
| `schema_migrations` | Applied SQL versions. |

## HTTP server (`internal/server`)

`NewHandler` создаёт `http.ServeMux`, затем оборачивает его в порядке:

```text
requestID → structured request log → security headers → CORS → metrics → mux
```

Маршруты:

| Route | Handler |
|---|---|
| `GET /health/live` | Процесс жив. |
| `GET /health/ready` | PostgreSQL отвечает. |
| `GET /metrics` | Prometheus text exposition. |
| `/api/v1/auth/*` | `auth.HTTPHandler`. |
| `/api/v1/agent-tokens*` | `agenttokens.HTTPHandler`. |
| `/api/v1/agent/connect` | WebSocket `gateway.AgentConnectHandler`. |
| `/api/v1/agents` | `agents.HTTPHandler`. |
| `/api/v1/tunnels` | `tunnels.HTTPHandler`. |
| `/` | `tunnels.PublicHandler` для public host. |

### Middleware

- `requestID` сохраняет безопасный входящий `X-Request-ID` или создаёт
  `req_<24 hex>`; ID возвращается в response и logs.
- `requestLog` пишет JSON: method, path, remote IP, request ID и duration.
  Headers, tokens, cookies и request bodies не логируются.
- `cors` принимает только настроенные origins и корректно отвечает preflight.
- `securityHeaders` применяется к control plane, но не меняет response
  proxied local application.
- `Metrics` считает HTTP request/in-flight/duration с низкой cardinality
  labels (`method`, route group), gateway sessions и in-memory traffic.

## Auth (`internal/auth`)

`Service` владеет PostgreSQL pool, access/refresh secrets и TTL.

| Метод | Поведение |
|---|---|
| `Register` | Нормализует email, проверяет password, bcrypt hashes, inserts user. |
| `Login` | Проверяет hash/status, обновляет login timestamp, выдаёт token pair. |
| `Refresh` | Находит active refresh hash, revoke old token, выдаёт rotation pair. |
| `Logout` | Revokes refresh token; операция безопасна при absent token. |
| `UserFromAccessToken` | Проверяет signed JWT и активность user. |

`HTTPHandler` реализует `POST /register`, `/login`, `/refresh`, `/logout` и
`GET /me`. Login/register/refresh ограничены 10 попытками в минуту на IP.
Refresh token доставляется только как HttpOnly cookie: development использует
`refresh_token`, HTTPS production — `__Host-refresh_token`, `Secure` и
`SameSite=Strict`.

## Agent tokens (`internal/agenttokens`)

Opaque credential создаётся как random value с префиксом `tkn_`. База хранит
SHA-256 hash и короткий visible prefix; plaintext возвращается только на create.

| Метод | Поведение |
|---|---|
| `Create(userID, name)` | Валидирует name, генерирует token, сохраняет hash. |
| `List(userID)` | Возвращает owner-scoped metadata без secret. |
| `Revoke(userID, tokenID)` | Ставит `revoked_at`, возвращает hash для disconnect. |

API: `GET/POST /api/v1/agent-tokens`, `DELETE /api/v1/agent-tokens/{id}`.
После revoke `SessionManager.DisconnectTokenHash` отменяет все WebSocket
sessions, использовавшие этот credential.

## Agents (`internal/agents`)

`Agent` — API model: ID, instance ID, hostname, OS, architecture, version,
connected state и last seen timestamp.

| Метод | Поведение |
|---|---|
| `Connect(rawToken, hello)` | Валидирует non-revoked token, upserts agent instance, marks connected. |
| `Touch(id)` | Обновляет `last_seen_at`. |
| `Disconnect(id)` | Marks persisted agent offline. |
| `List(userID)` | Возвращает только agents owner. |

`GET /api/v1/agents` требует Bearer access JWT.

## Gateway (`internal/gateway`)

### SessionManager

`SessionManager` — thread-safe in-memory owner активных WebSocket agents.
`Session` хранит `ID`, persistent `AgentID`, token hash, `Outbound` bounded queue
и cancellation function. `Response` — correlated public response.

Ключевые методы: `Register`, `Remove`, `Send`, `SendBinary`, context-aware
`SendContext`, `RegisterRequest`, `DeliverResponse`, `CancelRequest`,
`DisconnectTokenHash`, `CloseAll`, `MetricsSnapshot` и `AgentID`.

`Remove` отменяет все pending requests конкретной session ответом `502`; это
исключает ожидание полного timeout при disconnect agent.

### AgentConnectHandler

`ServeHTTP` требует `Authorization: Bearer tkn_*`, валидирует hash в БД,
upgrade до WebSocket и выполняет `handshake`.

Handshake принимает `client_hello`, проверяет protocol version, создаёт `ses_*`,
registers session и отправляет `server_hello`. `serveSession` обрабатывает:

- heartbeat `ping`/`pong`;
- `open_tunnel` и `tunnel_opened`;
- response start/body binary chunks/end;
- bounded concurrent response assemblies;
- protocol violations, body limit 32 MiB и cleanup on close.

## Tunnels (`internal/tunnels`)

`Service` сохраняет lifecycle tunnel. `ReopenForSession` либо возвращает
последний закрытый tunnel для того же agent/local address, либо создаёт новый
DNS-safe random subdomain. `CloseAgentTunnels` закрывает persistent routes при
disconnect; `List` returns owner-scoped tunnels.

`Registry` — runtime map active routes by subdomain/ID. Методы `Open`, `Get`,
`Close`, `CloseSession`, `Count`, `RecordTraffic`, `TrafficMetrics` защищены
mutex. Traffic counters intentionally reset on backend restart.

`ParsePublicHost` принимает ровно один DNS-safe label перед configured base
domain and ignores optional port.

### PublicHandler proxy flow

1. Parse request `Host`; unknown/malformed host → `404`.
2. Find active `Registry` tunnel; apply 120 requests/minute per tunnel+IP.
3. Generate request ID, send `http_request_start`, binary request chunks and
   `http_request_end` through owning session.
4. Wait for correlated `Response`, timeout, browser cancellation or disconnect.
5. Copy local status, response body and headers to public client.

`Cookie` and `Set-Cookie` are normal end-to-end headers and are preserved.
Hop-by-hop headers (`Connection`, `Upgrade`, `Transfer-Encoding` and headers
named by `Connection`) are stripped at both boundaries. Gateway overwrites
untrusted forwarded headers with `X-Forwarded-For`, `X-Forwarded-Host`,
`X-Forwarded-Proto`, `X-Tunnel-Request-ID`.

## Protocol (`internal/protocol`)

Text messages use JSON `Envelope{type,payload}`. Main types: `client_hello`,
`server_hello`, `ping`, `pong`, `open_tunnel`, `tunnel_opened`, `close_tunnel`,
`http_request_start/end`, `http_response_start/end`, `cancel_request`.

Large bodies use `BinaryFrame`: one-byte type, request ID, uint32 sequence and
payload. `MarshalBinary`/`ParseBinaryFrame` enforce 32 KiB payload limit.

## Error behaviour and limits

| Condition | Result |
|---|---|
| Unknown public host/tunnel | `404`. |
| Agent unavailable/disconnected | `502`. |
| Agent response timeout | `504`. |
| Body over 32 MiB | `413`. |
| Public rate limit exceeded | `429` with `Retry-After`. |
| Invalid agent token/protocol | WebSocket rejected/closed. |

## Tests

`go test ./...` runs unit tests, PostgreSQL integration tests, HTTP handler,
WebSocket protocol, public proxy, security and metrics tests. PostgreSQL tests
use `TEST_DATABASE_URL` or the local Compose database (`localhost:15432`).
Current aggregate backend statement coverage is recorded in the implementation
plan. Run:

```powershell
Set-Location backend
go test ./...
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```
