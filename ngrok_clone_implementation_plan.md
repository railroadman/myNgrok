# Self-hosted ngrok-like Tunnel Platform — Implementation Plan

> ## Implementation tracker (authoritative current state)
>
> Status notation: `[ ]` not started, `[-]` in progress, `[x]` completed and
> verified, `[!]` blocked. Update this tracker and the applicable Epic task after
> every implementation change. A task is `[x]` only when its required backend,
> agent, and/or frontend tests pass.
>
> **Mandatory test policy:** every task must add or update automated tests for the
> behavior it changes. Backend: unit tests plus PostgreSQL/integration tests for
> persistence or HTTP behavior. Agent: unit tests plus protocol/local-proxy tests.
> Frontend: component/store/API-client tests for each view or user flow. A task
> spanning multiple applications requires tests in every affected application;
> end-to-end tests cover the complete flow. Manual checks complement tests but do
> not replace them.
>
> **Mandatory documentation policy:** every completed task must update the
> applicable project documentation (README, runbook, API/protocol reference, or
> architecture notes). Documentation must describe the delivered behavior, how to
> use it, configuration and known limitations. Review and refresh it alongside
> tests before marking a task `[x]`.
>
> **Execution policy:** Codex proceeds autonomously through the ordered Epic
> tasks. It reports completed work and only stops for an external dependency,
> a safety-sensitive decision, or a genuine ambiguity that cannot be resolved
> from the repository and this plan.
>
> **Coverage quality gate:** aggregate Go statement coverage must reach at
> least **80%** separately for backend and agent before final project handoff.
> Coverage remediation and integration-test stabilization are scheduled after
> deployment tasks, at the user's direction. Raise coverage through meaningful
> tests; do not exclude production packages or lower the measured scope.
> Latest measurement: backend **86.2%**, agent **80.6%**. Both codebases meet
> the coverage quality gate.
>
> **Current focus:** All implementation and coverage gates are complete.
>
> **Project progress:** **102 / 102 tasks complete (100.0%)**.
> Formula: `completed [x] Epic tasks / 102 total Epic tasks × 100`, rounded to one
> decimal place. Do not count `[−]` in-progress tasks. Recalculate and update this
> line whenever a task status changes; include the current percentage in every
> implementation handoff or progress report.
>
> - [x] Backend Go module, typed environment config, JSON logger, graceful HTTP
>   server, and `/health/live` + `/health/ready` tests.
> - [x] Agent Go module with `version` and validated `http` CLI skeleton.
> - [x] Root development artifacts: `.env.example`, `.gitignore`, Makefile,
>   Docker Compose PostgreSQL service, README, and protocol documentation stub.
> - [x] PostgreSQL driver, connection pool, migration runner, and immutable initial migration.
> - [x] Vue + TypeScript + Vite application shell.
> - [x] Docker Compose backend/frontend services build and start; PostgreSQL becomes
>   healthy and backend readiness returns `{"status":"ok"}` over the Compose network.
> - [x] Phase 1 — user authentication (register/login/refresh/logout/me and frontend UI).
> - [x] Phase 2 — agent tokens.
> - [x] Phase 3 — WebSocket agent connectivity and tunnel registration.
> - [x] Phase 4 — HTTP forwarding, multiplexing, streaming, and reliability.
> - [x] Phase 5 — frontend completion, observability, security, and deployment.

## 1. Цель проекта

Создать минимальный, но архитектурно правильный self-hosted аналог ngrok, который позволяет пользователю открыть локальный HTTP-сервис в интернет через публичный gateway.

Базовый сценарий:

```text
Internet
   |
   v
Public Gateway
   ^
   | persistent outbound connection
   |
Agent on local machine
   |
   v
http://127.0.0.1:8080
```

Пользователь устанавливает agent на локальный компьютер, получает токен в web-интерфейсе и запускает:

```bash
tunnel-agent http 8080 --token <TOKEN>
```

После подключения gateway выдает публичный URL:

```text
https://abc123.example.com
```

Внешний запрос:

```text
GET https://abc123.example.com/api/health
```

должен быть передан через уже установленное соединение agent → gateway на:

```text
http://127.0.0.1:8080/api/health
```

и ответ локального приложения должен вернуться наружу через gateway.

---

# 2. Scope проекта

## 2.1. MVP

В первую версию входят:

- регистрация пользователя;
- вход пользователя;
- access token / refresh token для web API;
- agent token для подключения клиента;
- web-интерфейс;
- создание и отзыв agent tokens;
- запуск agent на локальном компьютере;
- persistent connection agent → gateway;
- регистрация HTTP tunnel;
- генерация случайного subdomain;
- маршрутизация запросов по `Host`;
- forwarding HTTP methods;
- forwarding request headers;
- forwarding request body;
- forwarding response status;
- forwarding response headers;
- forwarding response body;
- поддержка нескольких одновременных HTTP requests;
- heartbeats;
- reconnect agent;
- graceful shutdown;
- PostgreSQL;
- structured logs;
- базовые metrics;
- health/readiness endpoints;
- Docker deployment;
- TLS на gateway;
- минимальный Vue frontend.

## 2.2. Не входит в первый MVP

Отложить:

- raw TCP tunnels;
- UDP;
- SSH tunneling;
- custom domains;
- wildcard user-owned domains;
- custom TLS certificates;
- traffic replay;
- request inspection UI;
- request history;
- billing;
- payment system;
- quotas по тарифам;
- multi-region routing;
- active-active gateway cluster;
- Kubernetes;
- SSO;
- OAuth providers;
- teams;
- organizations;
- RBAC;
- webhooks;
- permanent reserved domains;
- end-to-end encrypted opaque tunnels;
- HTTP/3;
- QUIC;
- gRPC tunneling;
- WebSocket passthrough как отдельная feature;
- public file sharing.

---

# 3. Основные архитектурные блоки

Система делится на четыре крупных части:

```text
┌───────────────────────────────────────────────────────────────┐
│                        Vue Frontend                           │
│                                                               │
│ Login / Register / Dashboard / Tokens / Tunnels / Agents      │
└───────────────────────────────┬───────────────────────────────┘
                                │ HTTPS REST API
                                v
┌───────────────────────────────────────────────────────────────┐
│                     Control Plane Backend                     │
│                              Go                               │
│                                                               │
│ Auth / Users / Tokens / Tunnel metadata / Agent metadata      │
└───────────────────────────────┬───────────────────────────────┘
                                │
                                v
                         ┌──────────────┐
                         │ PostgreSQL   │
                         └──────────────┘

Internet
   |
   v
┌───────────────────────────────────────────────────────────────┐
│                         Gateway                               │
│                            Go                                 │
│                                                               │
│ Public HTTP ingress                                           │
│ Host routing                                                  │
│ Tunnel session manager                                        │
│ Request multiplexing                                          │
│ Agent connection endpoint                                     │
└───────────────────────────────┬───────────────────────────────┘
                                │ persistent secure connection
                                v
┌───────────────────────────────────────────────────────────────┐
│                         Agent                                 │
│                            Go                                 │
│                                                               │
│ Auth                                                          │
│ Tunnel registration                                           │
│ Request forwarding                                            │
│ Local HTTP client                                              │
│ Retry / reconnect / heartbeat                                 │
└───────────────────────────────┬───────────────────────────────┘
                                │
                                v
                         localhost:PORT
```

Для MVP Control Plane Backend и Gateway допустимо запускать в одном Go-процессе.

Логически модули должны быть разделены так, чтобы позже их можно было вынести в отдельные сервисы без переписывания доменной логики.

---

# 4. Репозиторий и структура проекта

Рекомендуемая структура monorepo:

```text
tunnel-platform/
├── README.md
├── Makefile
├── docker-compose.yml
├── .env.example
├── .gitignore
│
├── backend/
│   ├── cmd/
│   │   └── server/
│   │       └── main.go
│   │
│   ├── internal/
│   │   ├── config/
│   │   ├── auth/
│   │   ├── users/
│   │   ├── agents/
│   │   ├── tokens/
│   │   ├── tunnels/
│   │   ├── gateway/
│   │   ├── protocol/
│   │   ├── database/
│   │   ├── middleware/
│   │   ├── telemetry/
│   │   └── server/
│   │
│   ├── migrations/
│   ├── tests/
│   ├── go.mod
│   └── go.sum
│
├── agent/
│   ├── cmd/
│   │   └── tunnel-agent/
│   │       └── main.go
│   ├── internal/
│   │   ├── cli/
│   │   ├── config/
│   │   ├── client/
│   │   ├── tunnel/
│   │   ├── protocol/
│   │   ├── localproxy/
│   │   ├── reconnect/
│   │   └── telemetry/
│   ├── tests/
│   ├── go.mod
│   └── go.sum
│
├── frontend/
│   ├── src/
│   │   ├── api/
│   │   ├── components/
│   │   ├── layouts/
│   │   ├── router/
│   │   ├── stores/
│   │   ├── views/
│   │   ├── types/
│   │   └── main.ts
│   ├── public/
│   ├── package.json
│   └── vite.config.ts
│
├── protocol/
│   ├── README.md
│   ├── messages.md
│   ├── errors.md
│   └── examples.md
│
├── deploy/
│   ├── docker/
│   ├── nginx/
│   └── systemd/
│
└── docs/
    ├── architecture.md
    ├── api.md
    ├── security.md
    ├── operations.md
    └── development.md
```

---

# 5. Backend / Control Plane

## 5.1. Ответственность backend

Backend отвечает за:

- регистрацию;
- login;
- logout;
- refresh token;
- управление пользователями;
- управление agent tokens;
- хранение tunnel metadata;
- хранение agent metadata;
- отображение online/offline состояния;
- API для Vue frontend;
- авторизацию gateway connections;
- audit metadata;
- управление revocation.

Backend не должен непосредственно заниматься проксированием HTTP body, если gateway позднее станет отдельным процессом.

---

# 6. Backend — конфигурация

## 6.1. Environment variables

Минимальный набор:

```text
APP_ENV=development

HTTP_ADDR=:8080
PUBLIC_BASE_DOMAIN=tunnel.example.com

DATABASE_URL=postgres://user:password@postgres:5432/tunnel?sslmode=disable

JWT_ACCESS_SECRET=...
JWT_REFRESH_SECRET=...

ACCESS_TOKEN_TTL=15m
REFRESH_TOKEN_TTL=720h

AGENT_TOKEN_PREFIX=tkn_

TUNNEL_SESSION_TIMEOUT=60s
TUNNEL_HEARTBEAT_INTERVAL=20s
TUNNEL_REQUEST_TIMEOUT=60s

LOG_LEVEL=info
```

## 6.2. Требования к config module

Config:

- читается один раз при старте;
- валидируется;
- приложение не запускается при критической ошибке конфигурации;
- secrets не логируются;
- конфигурация должна быть типизированной.

Пример структуры:

```go
type Config struct {
    Environment string
    HTTP        HTTPConfig
    Database    DatabaseConfig
    Auth        AuthConfig
    Tunnel      TunnelConfig
    Logging     LoggingConfig
}
```

---

# 7. PostgreSQL — схема данных

## 7.1. users

```sql
users
-----
id UUID PRIMARY KEY
email VARCHAR(320) UNIQUE NOT NULL
password_hash TEXT NOT NULL
status VARCHAR(32) NOT NULL
created_at TIMESTAMPTZ NOT NULL
updated_at TIMESTAMPTZ NOT NULL
last_login_at TIMESTAMPTZ NULL
```

Status:

```text
active
disabled
deleted
```

---

## 7.2. refresh_tokens

```sql
refresh_tokens
--------------
id UUID PRIMARY KEY
user_id UUID NOT NULL
token_hash TEXT NOT NULL
expires_at TIMESTAMPTZ NOT NULL
revoked_at TIMESTAMPTZ NULL
created_at TIMESTAMPTZ NOT NULL
user_agent TEXT NULL
ip_address INET NULL
```

Индексы:

```text
user_id
token_hash UNIQUE
expires_at
```

---

## 7.3. agent_tokens

```sql
agent_tokens
------------
id UUID PRIMARY KEY
user_id UUID NOT NULL
name VARCHAR(128) NOT NULL
token_prefix VARCHAR(32) NOT NULL
token_hash TEXT NOT NULL
created_at TIMESTAMPTZ NOT NULL
last_used_at TIMESTAMPTZ NULL
expires_at TIMESTAMPTZ NULL
revoked_at TIMESTAMPTZ NULL
```

Никогда не сохранять исходный token.

Полный token показывается пользователю только один раз.

---

## 7.4. agents

```sql
agents
------
id UUID PRIMARY KEY
user_id UUID NOT NULL
agent_token_id UUID NOT NULL
instance_id UUID NOT NULL
name VARCHAR(128) NULL
hostname VARCHAR(255) NULL
os VARCHAR(64) NULL
arch VARCHAR(64) NULL
version VARCHAR(64) NULL
status VARCHAR(32) NOT NULL
connected_at TIMESTAMPTZ NULL
last_seen_at TIMESTAMPTZ NULL
disconnected_at TIMESTAMPTZ NULL
created_at TIMESTAMPTZ NOT NULL
```

Status:

```text
online
offline
revoked
```

---

## 7.5. tunnels

```sql
tunnels
-------
id UUID PRIMARY KEY
user_id UUID NOT NULL
agent_id UUID NULL
public_id VARCHAR(64) UNIQUE NOT NULL
subdomain VARCHAR(128) UNIQUE NOT NULL
protocol VARCHAR(16) NOT NULL
local_host VARCHAR(255) NOT NULL
local_port INTEGER NOT NULL
status VARCHAR(32) NOT NULL
created_at TIMESTAMPTZ NOT NULL
connected_at TIMESTAMPTZ NULL
disconnected_at TIMESTAMPTZ NULL
last_request_at TIMESTAMPTZ NULL
```

MVP protocol:

```text
http
```

Status:

```text
pending
online
offline
closed
```

---

## 7.6. tunnel_sessions

Опционально для MVP, обязательно для последующего аудита.

```sql
tunnel_sessions
---------------
id UUID PRIMARY KEY
tunnel_id UUID NOT NULL
agent_id UUID NOT NULL
session_id UUID NOT NULL
connected_at TIMESTAMPTZ NOT NULL
disconnected_at TIMESTAMPTZ NULL
disconnect_reason TEXT NULL
remote_ip INET NULL
```

---

# 8. Database layer

Нужно разделить:

```text
repository
service
transport
```

Пример:

```text
users/
├── model.go
├── repository.go
├── service.go
├── handler.go
└── errors.go
```

Repository interface:

```go
type UserRepository interface {
    Create(ctx context.Context, user *User) error
    GetByID(ctx context.Context, id uuid.UUID) (*User, error)
    GetByEmail(ctx context.Context, email string) (*User, error)
    UpdateLastLogin(ctx context.Context, id uuid.UUID) error
}
```

Не возвращать SQL errors напрямую наружу.

---

# 9. Auth subsystem

## 9.1. User authentication

Использовать:

- email;
- password;
- secure password hashing;
- access JWT;
- refresh token.

Flow:

```text
POST /api/v1/auth/register
POST /api/v1/auth/login
POST /api/v1/auth/refresh
POST /api/v1/auth/logout
GET  /api/v1/auth/me
```

---

## 9.2. Password requirements

Минимум:

- length >= 10;
- password hash через Argon2id или bcrypt;
- plaintext password никогда не логировать;
- одинаковый generic response для части auth errors желательно предусмотреть позднее.

---

## 9.3. Access JWT

Claims:

```json
{
  "sub": "user-uuid",
  "type": "access",
  "iat": 0,
  "exp": 0
}
```

Access token:

- короткий TTL;
- используется только frontend/API.

Agent не должен подключаться через пользовательский JWT.

---

## 9.4. Agent token

Agent token — отдельный opaque credential.

Пример:

```text
tkn_J7Skz5xK1...
```

В базе:

```text
prefix = tkn_J7Sk
hash   = SHA-256(full_token)
```

При connect:

```text
Authorization: Bearer tkn_J7Skz5xK1...
```

или auth message внутри протокола.

Рекомендация MVP:

- передавать token при WebSocket handshake через Authorization header;
- TLS обязателен;
- gateway сравнивает hash.

---

# 10. REST API

## 10.1. Auth

```text
POST /api/v1/auth/register
POST /api/v1/auth/login
POST /api/v1/auth/refresh
POST /api/v1/auth/logout
GET  /api/v1/auth/me
```

---

## 10.2. Agent Tokens

```text
GET    /api/v1/agent-tokens
POST   /api/v1/agent-tokens
DELETE /api/v1/agent-tokens/{id}
```

Create request:

```json
{
  "name": "Home PC"
}
```

Create response:

```json
{
  "id": "uuid",
  "name": "Home PC",
  "token": "tkn_xxxxxxxxx",
  "createdAt": "..."
}
```

Важно:

`token` присутствует только в create response.

---

## 10.3. Agents

```text
GET /api/v1/agents
GET /api/v1/agents/{id}
```

Response:

```json
{
  "id": "uuid",
  "name": "home-linux",
  "hostname": "genmachine-pc",
  "os": "linux",
  "arch": "amd64",
  "version": "0.1.0",
  "status": "online",
  "connectedAt": "...",
  "lastSeenAt": "..."
}
```

---

## 10.4. Tunnels

```text
GET    /api/v1/tunnels
GET    /api/v1/tunnels/{id}
DELETE /api/v1/tunnels/{id}
```

Создание tunnel в MVP рекомендуется делать через agent protocol, а не REST.

Причина:

локальный порт известен agent и tunnel должен быть связан с активной session.

---

# 11. Gateway

## 11.1. Ответственность gateway

Gateway отвечает за:

- endpoint подключения agent;
- authentication agent;
- session registry;
- tunnel registry;
- public HTTP ingress;
- поиск tunnel по hostname;
- передачу request agent;
- ожидание response;
- возврат response пользователю;
- cancellation;
- timeout;
- heartbeat;
- cleanup disconnected sessions.

---

# 12. Public DNS

Нужен wildcard DNS:

```text
*.tunnel.example.com -> GATEWAY_PUBLIC_IP
```

Пример:

```text
abc123.tunnel.example.com
dev1.tunnel.example.com
x7fh20.tunnel.example.com
```

Все subdomain идут на один gateway.

Gateway использует HTTP Host:

```text
Host: abc123.tunnel.example.com
```

и ищет tunnel:

```text
subdomain = abc123
```

---

# 13. TLS

Для публичных tunnels нужен wildcard certificate:

```text
*.tunnel.example.com
```

или reverse proxy перед gateway.

MVP варианты:

### Вариант A

```text
Internet
   |
 Nginx/Caddy
   |
 Go Gateway
```

TLS termination делает Caddy/Nginx.

### Вариант B

Go gateway сам слушает HTTPS.

Для первой production-like версии проще:

```text
Caddy -> Go
```

Но приложение должно корректно работать и без внешнего proxy в development.

---

# 14. Agent connection

## 14.1. Transport

Для MVP использовать WebSocket поверх TLS:

```text
wss://gateway.example.com/api/v1/agent/connect
```

Причины:

- просто реализовать;
- NAT-friendly;
- proxy-friendly;
- TLS;
- bidirectional;
- поддерживает binary frames;
- нормально подходит для MVP.

---

# 15. Tunnel protocol

Нужно вынести protocol specification в:

```text
/protocol/messages.md
```

Каждое сообщение содержит:

```json
{
  "type": "...",
  "requestId": "...",
  "payload": {}
}
```

Но для production throughput лучше использовать typed envelopes и binary payload.

Для MVP допускается JSON metadata + binary body chunks.

---

# 16. Protocol messages

## 16.1. ClientHello

Agent → Gateway:

```json
{
  "type": "client_hello",
  "protocolVersion": 1,
  "agentVersion": "0.1.0",
  "instanceId": "uuid",
  "hostname": "genmachine-pc",
  "os": "linux",
  "arch": "amd64"
}
```

Gateway:

```json
{
  "type": "server_hello",
  "protocolVersion": 1,
  "sessionId": "uuid",
  "heartbeatIntervalSeconds": 20
}
```

---

## 16.2. OpenTunnel

Agent:

```json
{
  "type": "open_tunnel",
  "requestId": "uuid",
  "protocol": "http",
  "localHost": "127.0.0.1",
  "localPort": 8080
}
```

Gateway:

```json
{
  "type": "tunnel_opened",
  "requestId": "uuid",
  "tunnelId": "uuid",
  "publicUrl": "https://abc123.tunnel.example.com"
}
```

---

## 16.3. CloseTunnel

```json
{
  "type": "close_tunnel",
  "tunnelId": "uuid"
}
```

---

## 16.4. HTTP Request Start

Gateway → Agent:

```json
{
  "type": "http_request_start",
  "requestId": "uuid",
  "tunnelId": "uuid",
  "method": "POST",
  "path": "/api/items?limit=10",
  "headers": {
    "content-type": ["application/json"],
    "user-agent": ["curl/8"]
  },
  "contentLength": 125
}
```

---

## 16.5. HTTP Request Body Chunk

Для больших body не отправлять весь body внутри JSON.

Нужны chunks:

```text
REQUEST_BODY_CHUNK
requestId
sequence
binary payload
```

В упрощенном JSON MVP можно использовать base64 только как временный этап.

Рекомендуется сразу сделать binary frame format.

---

## 16.6. HTTP Request End

```json
{
  "type": "http_request_end",
  "requestId": "uuid"
}
```

---

## 16.7. HTTP Response Start

Agent → Gateway:

```json
{
  "type": "http_response_start",
  "requestId": "uuid",
  "status": 200,
  "headers": {
    "content-type": ["application/json"]
  }
}
```

---

## 16.8. HTTP Response Body Chunk

Binary chunks.

---

## 16.9. HTTP Response End

```json
{
  "type": "http_response_end",
  "requestId": "uuid"
}
```

---

## 16.10. Request Error

```json
{
  "type": "request_error",
  "requestId": "uuid",
  "code": "LOCAL_CONNECTION_REFUSED",
  "message": "local service is unavailable"
}
```

Gateway преобразует это, например, в:

```text
502 Bad Gateway
```

---

# 17. Protocol error codes

Минимально:

```text
AUTH_FAILED
TOKEN_REVOKED
PROTOCOL_VERSION_UNSUPPORTED
INVALID_MESSAGE
INVALID_TUNNEL
TUNNEL_NOT_FOUND
TUNNEL_ALREADY_EXISTS
LOCAL_CONNECTION_REFUSED
LOCAL_TIMEOUT
REQUEST_TIMEOUT
REQUEST_CANCELLED
BODY_TOO_LARGE
INTERNAL_ERROR
```

---

# 18. Heartbeat

Agent:

```json
{
  "type": "ping",
  "timestamp": 123456789
}
```

Gateway:

```json
{
  "type": "pong",
  "timestamp": 123456789
}
```

Правила:

- heartbeat каждые N секунд;
- если session молчит дольше session timeout — disconnected;
- все tunnels session переводятся offline;
- pending requests завершаются ошибкой;
- DB получает disconnected state.

---

# 19. Session Manager

Gateway должен иметь in-memory registry:

```go
type SessionManager struct {
    sessions map[uuid.UUID]*AgentSession
    tunnels  map[string]*TunnelSession
}
```

Но доступ — thread-safe.

Использовать:

- mutex;
- RWMutex;
- sync.Map только если есть реальная причина.

Предпочтительно:

```text
SessionManager
  ├── RegisterSession
  ├── RemoveSession
  ├── RegisterTunnel
  ├── RemoveTunnel
  ├── FindTunnelByHost
  └── FindSession
```

---

# 20. AgentSession

Пример ответственности:

```go
type AgentSession struct {
    ID         uuid.UUID
    UserID     uuid.UUID
    AgentID    uuid.UUID
    Conn       *websocket.Conn

    SendQueue  chan Message
    Pending    map[string]*PendingRequest

    ctx        context.Context
    cancel     context.CancelFunc
}
```

Важно:

не писать в WebSocket одновременно из нескольких goroutine без coordination.

Нужен один writer loop:

```text
goroutines
   |
   v
sendQueue
   |
   v
single websocket writer
```

---

# 21. Request Multiplexing

Один agent connection должен обслуживать много HTTP requests.

Пример:

```text
WebSocket
  |
  + request A
  + request B
  + request C
```

Каждый запрос имеет уникальный:

```text
requestId
```

Gateway хранит:

```go
map[RequestID]*PendingRequest
```

PendingRequest:

```go
type PendingRequest struct {
    ResponseStarted chan ResponseMeta
    Body            io.Reader
    Done            chan error
}
```

---

# 22. Backpressure

Это критическая часть.

Нельзя бесконтрольно читать внешний request body и складывать его в память.

Нужно:

```text
external request
      |
      v
bounded buffer
      |
      v
websocket
      |
      v
agent
```

MVP лимиты:

```text
max request body
max response body
max concurrent requests per tunnel
max send queue
```

Например конфигурационно:

```text
MAX_REQUEST_BODY_MB
MAX_CONCURRENT_REQUESTS_PER_TUNNEL
MAX_FRAME_SIZE
```

Числа должны быть определены отдельным конфигом, а не захардкожены в нескольких местах.

---

# 23. Cancellation

Если внешний клиент закрыл соединение:

```text
Browser disconnect
      |
Gateway ctx cancelled
      |
CANCEL_REQUEST
      |
Agent cancels local HTTP request
```

Protocol:

```json
{
  "type": "cancel_request",
  "requestId": "uuid"
}
```

Agent должен создавать локальный request через context:

```go
req = req.WithContext(ctx)
```

---

# 24. HTTP forwarding rules

Forward:

- method;
- path;
- query string;
- most headers;
- body;
- status;
- response headers;
- response body.

Не forward напрямую hop-by-hop headers:

```text
Connection
Proxy-Connection
Keep-Alive
Proxy-Authenticate
Proxy-Authorization
TE
Trailer
Transfer-Encoding
Upgrade
```

Host handling надо определить явно.

Рекомендуемое поведение:

локальному приложению передавать:

```text
Host: 127.0.0.1:8080
```

и добавлять:

```text
X-Forwarded-Host
X-Forwarded-Proto
X-Forwarded-For
X-Tunnel-Request-ID
```

---

# 25. Public request flow

Полный flow:

```text
1. Browser -> gateway
2. Gateway reads Host
3. Gateway resolves subdomain
4. SessionManager finds tunnel
5. Validate tunnel online
6. Generate requestId
7. Register PendingRequest
8. Send HTTP_REQUEST_START
9. Stream request body
10. Send HTTP_REQUEST_END
11. Agent receives request
12. Agent sends request to localhost
13. Agent sends HTTP_RESPONSE_START
14. Agent streams response body
15. Agent sends HTTP_RESPONSE_END
16. Gateway writes status/headers/body to browser
17. Cleanup PendingRequest
```

---

# 26. Gateway HTTP error mapping

```text
unknown hostname        -> 404
known tunnel offline    -> 502 or 503
agent disconnected      -> 502
local connection refused-> 502
gateway timeout         -> 504
request too large       -> 413
too many requests       -> 429
invalid protocol        -> 502
internal error          -> 500
```

Создать единый error mapping module.

---

# 27. Subdomain generator

MVP:

```text
[a-z0-9]{8}
```

Пример:

```text
f7k2m1qa
```

Требования:

- cryptographically secure random;
- уникальность проверяется через DB unique constraint;
- retry при collision;
- lowercase;
- не использовать user input.

---

# 28. Agent

## 28.1. Основная задача agent

Agent:

- принимает CLI параметры;
- загружает token;
- подключается к gateway;
- authenticates;
- отправляет metadata;
- регистрирует tunnel;
- получает requests;
- проксирует в localhost;
- возвращает response;
- держит heartbeat;
- восстанавливает соединение после разрыва.

---

# 29. Agent CLI

Команды:

```bash
tunnel-agent login --token ...
tunnel-agent http 8080
tunnel-agent http localhost:8080
tunnel-agent status
tunnel-agent version
```

MVP можно начать с:

```bash
tunnel-agent http 8080 --token ...
```

---

# 30. Agent config file

После `login`:

Linux:

```text
~/.config/tunnel-agent/config.toml
```

Windows:

```text
%APPDATA%\TunnelAgent\config.toml
```

Пример:

```toml
gateway = "wss://gateway.example.com/api/v1/agent/connect"
token = "tkn_xxxxx"

[defaults]
local_host = "127.0.0.1"
```

Permissions:

- Linux config file 0600;
- token не выводить в logs.

---

# 31. Agent instance identity

При первом запуске создать:

```text
instance_id UUID
```

Хранить локально.

Таким образом gateway понимает:

```text
это тот же agent installation
```

а не новая машина при каждом reconnect.

---

# 32. Agent connection lifecycle

```text
START
 |
load config
 |
validate CLI
 |
connect WSS
 |
authenticate
 |
client hello
 |
server hello
 |
open tunnel
 |
READY
 |
serve requests
 |
connection failure
 |
reconnect with backoff
```

---

# 33. Reconnect strategy

Exponential backoff:

```text
1s
2s
4s
8s
...
max 30s
```

Добавить jitter.

После reconnect agent должен открыть tunnel заново.

MVP может получить новый random subdomain после reconnect.

Следующий этап — persistent tunnel identity.

---

# 34. Local HTTP forwarding

Agent получает:

```text
method
path
headers
body
```

и строит URL:

```text
http://127.0.0.1:8080 + path
```

Использовать один reusable:

```go
http.Client
```

с connection pooling.

Не создавать новый Transport на каждый request.

---

# 35. Agent local errors

Обработать:

```text
connection refused
DNS failure
timeout
TLS local error
context cancelled
unexpected EOF
```

Для MVP local destination — HTTP.

Позже:

```text
--local-scheme https
```

---

# 36. Agent concurrency

Каждый внешний request можно выполнять в отдельной goroutine, но нужен semaphore:

```go
sem := make(chan struct{}, maxConcurrent)
```

Причина:

не позволять gateway создать неограниченное количество goroutine и локальных соединений.

---

# 37. Agent graceful shutdown

При Ctrl+C / SIGTERM:

```text
1. stop accepting new requests
2. notify gateway tunnel closing
3. cancel active requests
4. close websocket
5. exit
```

---

# 38. Frontend — Vue

## 38.1. Стек

Frontend:

- Vue;
- TypeScript;
- Vite;
- Vue Router;
- Pinia;
- fetch или Axios;
- минимальная component library либо собственный UI.

Не смешивать backend types вручную по всему приложению.

Выделить:

```text
src/types
src/api
```

---

# 39. Frontend routes

```text
/register
/login

/app
/app/dashboard
/app/tokens
/app/agents
/app/tunnels
/app/settings
```

---

# 40. Frontend Layout

```text
┌─────────────────────────────────────────────┐
│ Header                                      │
├──────────────┬──────────────────────────────┤
│ Sidebar      │ Page                         │
│              │                              │
│ Dashboard    │                              │
│ Tokens       │                              │
│ Agents       │                              │
│ Tunnels      │                              │
│ Settings     │                              │
└──────────────┴──────────────────────────────┘
```

---

# 41. Register page

Fields:

```text
email
password
confirm password
```

Behavior:

- client validation;
- backend validation;
- redirect to dashboard;
- proper errors.

---

# 42. Login page

Fields:

```text
email
password
```

После login:

```text
access token
refresh mechanism
current user
```

Не хранить refresh token в localStorage, если backend использует secure HttpOnly cookie.

Рекомендованный вариант:

```text
access token -> memory
refresh token -> Secure HttpOnly SameSite cookie
```

---

# 43. Dashboard

Показывать:

```text
Agents online
Active tunnels
Agent tokens
Recent connections
```

MVP можно без charts.

---

# 44. Tokens page

Функции:

- список tokens;
- create token;
- revoke token;
- copy generated token;
- warning, что token показывается один раз.

Create dialog:

```text
Token name: Home PC
```

После create:

```text
tkn_...
```

UI должен явно написать:

```text
Сохраните token сейчас. Позже он не будет показан снова.
```

---

# 45. Agents page

Columns:

```text
Name
Hostname
OS
Version
Status
Connected
Last Seen
```

Online indicator.

---

# 46. Tunnels page

Columns:

```text
Public URL
Local destination
Protocol
Agent
Status
Created
Last Request
```

Actions:

```text
copy URL
close tunnel
```

---

# 47. Frontend API layer

Пример:

```text
src/api/
├── client.ts
├── auth.ts
├── agentTokens.ts
├── agents.ts
└── tunnels.ts
```

`client.ts`:

- base URL;
- auth header;
- retry refresh on 401;
- standard API error parsing.

---

# 48. API response format

Унифицировать.

Success:

```json
{
  "data": {}
}
```

List:

```json
{
  "data": [],
  "meta": {
    "page": 1,
    "pageSize": 20,
    "total": 100
  }
}
```

Error:

```json
{
  "error": {
    "code": "INVALID_CREDENTIALS",
    "message": "Invalid email or password",
    "requestId": "..."
  }
}
```

---

# 49. Backend middleware

Нужны:

```text
Request ID
Access logging
Panic recovery
CORS
Authentication
Rate limit
Security headers
```

---

# 50. Request ID

Каждому REST и public tunnel request:

```text
X-Request-ID
```

Если клиент прислал подозрительный/слишком длинный — генерировать новый.

Логи связывать через request ID.

---

# 51. Logging

Structured JSON logs.

Поля:

```text
timestamp
level
message
request_id
user_id
agent_id
tunnel_id
session_id
remote_ip
duration_ms
error
```

Не логировать:

```text
password
agent token
refresh token
Authorization header
cookie
полное request body по умолчанию
```

---

# 52. Metrics

Минимальные Prometheus-like metrics:

```text
http_requests_total
http_request_duration_seconds

gateway_active_sessions
gateway_active_tunnels
gateway_active_requests

gateway_bytes_in_total
gateway_bytes_out_total

agent_connect_total
agent_disconnect_total

tunnel_request_total
tunnel_request_errors_total
```

---

# 53. Health endpoints

```text
GET /health/live
GET /health/ready
```

Liveness:

```text
process alive
```

Readiness:

```text
database reachable
gateway initialized
```

---

# 54. Security

## 54.1. TLS

Agent connection только:

```text
wss://
```

Production HTTP public tunnel:

```text
https://
```

---

## 54.2. Token storage

Agent token:

```text
random 256-bit value
```

В DB:

```text
hash only
```

Refresh tokens:

```text
hash only
```

---

## 54.3. Token revocation

При revoke token:

- backend marks revoked_at;
- новые agent connections запрещены;
- активную session желательно disconnect;
- все tunnels session offline.

Для MVP допустимо disconnect при следующем heartbeat check, но лучше сделать immediate session invalidation.

---

# 55. Abuse protection

Даже private/self-hosted версия должна иметь limits.

Нужны:

```text
max tunnels per user
max sessions per token
max request size
max response size
max concurrent requests
rate limit auth
rate limit public ingress
connection timeout
idle timeout
```

---

# 56. SSRF considerations

Agent по умолчанию должен проксировать только destination, указанное локальным пользователем.

Gateway не должен позволять внешнему HTTP request менять:

```text
localHost
localPort
```

То есть destination фиксируется tunnel metadata.

---

# 57. Header sanitization

Удалять/контролировать:

```text
Authorization
Proxy-Authorization
Connection
Upgrade
Forwarded
X-Forwarded-*
```

Политика должна быть документирована.

Для первого MVP Authorization внешнего запроса обычно нужно пропускать к локальному приложению.

Поэтому не удалять `Authorization` request header автоматически.

Отдельно не путать его с internal agent auth.

---

# 58. Database migrations

Использовать migration tool.

Структура:

```text
001_create_users.up.sql
001_create_users.down.sql
002_create_refresh_tokens.up.sql
...
```

Миграции:

- immutable;
- уже примененные migration файлы не изменять;
- новые изменения — новая migration.

---

# 59. Backend packages

Предлагаемое разбиение:

```text
internal/auth
internal/users
internal/agents
internal/tokens
internal/tunnels
internal/gateway
internal/protocol
internal/database
internal/httpapi
internal/telemetry
```

---

# 60. Domain errors

Не использовать arbitrary strings.

Пример:

```go
var (
    ErrNotFound          = errors.New("not found")
    ErrUnauthorized      = errors.New("unauthorized")
    ErrTokenRevoked      = errors.New("token revoked")
    ErrTunnelOffline     = errors.New("tunnel offline")
)
```

Transport mapping отдельно.

---

# 61. Context usage

Все I/O functions принимают:

```go
context.Context
```

Пример:

```go
func (s *Service) CreateToken(ctx context.Context, userID uuid.UUID, name string) (...)
```

Не использовать `context.Background()` внутри request-path logic без необходимости.

---

# 62. Timeouts

Обязательные timeout:

```text
HTTP server read header timeout
HTTP request timeout
database query timeout
agent handshake timeout
local connect timeout
local response header timeout
tunnel total timeout
write timeout
heartbeat timeout
```

Все должны быть configuration driven.

---

# 63. Public HTTP server

Нужно отделить public tunnel routes от control plane API.

Например:

```text
api.example.com
gateway.example.com
*.tunnel.example.com
```

MVP допустимо:

```text
gateway.example.com/api/v1/...
*.tunnel.example.com
```

Внутри server handler:

```text
if host == gateway.example.com:
    control plane
else if host endsWith .tunnel.example.com:
    public tunnel handler
else:
    404
```

---

# 64. WebSocket connection endpoint

```text
GET /api/v1/agent/connect
Upgrade: websocket
Authorization: Bearer tkn_...
```

До Upgrade:

1. parse token;
2. validate token;
3. find user;
4. reject revoked token;
5. upgrade;
6. create session.

---

# 65. Gateway internal components

```text
Gateway
├── AgentConnectionHandler
├── SessionManager
├── TunnelRegistry
├── PublicHTTPHandler
├── RequestDispatcher
├── ProtocolCodec
└── CleanupWorker
```

---

# 66. ProtocolCodec

Ответственность:

- serialize control messages;
- deserialize control messages;
- validate fields;
- validate protocol version;
- max frame size;
- reject unknown critical message types.

Не смешивать protocol decoding с domain logic.

---

# 67. PublicHTTPHandler

Pseudo flow:

```go
func ServeHTTP(w http.ResponseWriter, r *http.Request) {
    host := normalizeHost(r.Host)

    tunnel, ok := registry.FindByHost(host)
    if !ok {
        write404()
        return
    }

    if !tunnel.Online() {
        write503()
        return
    }

    dispatcher.Forward(w, r, tunnel)
}
```

---

# 68. RequestDispatcher

Ответственность:

- generate request id;
- create pending request;
- send request start;
- stream body;
- await response start;
- write status;
- stream response body;
- cleanup;
- timeout;
- cancel.

---

# 69. Agent localproxy module

```text
localproxy/
├── client.go
├── request.go
├── headers.go
└── errors.go
```

Основной interface:

```go
type Forwarder interface {
    Forward(ctx context.Context, req TunnelRequest) (*TunnelResponse, error)
}
```

В streaming implementation response должен быть потоковым, а не полностью загруженным в RAM.

---

# 70. MVP этапы реализации

## Phase 0 — Project bootstrap

### Backend

- создать Go module;
- config;
- logger;
- HTTP server;
- health endpoints;
- PostgreSQL connection;
- migrations.

### Agent

- Go module;
- CLI;
- config;
- version command.

### Frontend

- Vue + TypeScript;
- Vite;
- router;
- Pinia;
- base layout.

### Definition of Done

```text
docker compose up
```

поднимает:

```text
postgres
backend
frontend
```

Backend:

```text
GET /health/live -> 200
```

---

# 71. Phase 1 — User authentication

Tasks:

- migration users;
- user repository;
- register service;
- password hashing;
- login;
- JWT access;
- refresh token;
- logout;
- `/me`;
- auth middleware;
- frontend register/login;
- protected routes.

Tests:

- register success;
- duplicate email;
- bad email;
- bad password;
- login success;
- invalid password;
- expired access token;
- refresh;
- revoked refresh.

---

# 72. Phase 2 — Agent tokens

Backend:

- migration agent_tokens;
- generate secure token;
- hash token;
- create token;
- list tokens;
- revoke token;
- token authentication service.

Frontend:

- tokens page;
- create modal;
- one-time token display;
- copy button;
- revoke confirmation.

Tests:

- generated token unique;
- DB contains no plaintext token;
- revoked token invalid;
- foreign user cannot revoke another token.

---

# 73. Phase 3 — Agent WebSocket connection

Backend/Gateway:

- WebSocket endpoint;
- Authorization header;
- token validation;
- protocol version;
- ClientHello;
- ServerHello;
- session manager;
- heartbeat.

Agent:

- connect;
- token header;
- ClientHello;
- ServerHello;
- heartbeat;
- reconnect.

Definition of Done:

agent prints:

```text
Connected to gateway
Session: ...
```

Frontend agents page показывает agent online.

---

# 74. Phase 4 — Tunnel registration

Backend:

- migrations agents/tunnels;
- OpenTunnel message;
- subdomain generator;
- DB tunnel row;
- in-memory registry;
- TunnelOpened response.

Agent:

```bash
tunnel-agent http 8080
```

prints:

```text
Forwarding https://abc123.tunnel.example.com -> http://127.0.0.1:8080
```

Frontend:

- tunnels list;
- online status.

---

# 75. Phase 5 — Single HTTP request forwarding

Сначала сделать simplest working vertical slice.

Ограничения временного этапа:

- small body;
- body fully buffered;
- one request at a time.

Flow:

```text
GET /
```

через public URL возвращает local response.

Definition of Done:

```bash
python -m http.server 8080
tunnel-agent http 8080
curl https://abc123.tunnel.example.com
```

возвращает локальную страницу.

---

# 76. Phase 6 — Full HTTP semantics

Добавить:

- GET;
- POST;
- PUT;
- PATCH;
- DELETE;
- HEAD;
- OPTIONS;
- query strings;
- request headers;
- response headers;
- arbitrary status codes;
- request body;
- response body.

Tests через echo test server.

---

# 77. Phase 7 — Multiplexing

Убрать limitation one request at a time.

Добавить:

```text
requestId
pending requests registry
concurrent local requests
semaphore
```

Test:

```text
100 parallel requests
```

Проверить корректное соответствие response к requestId.

---

# 78. Phase 8 — Streaming

Убрать full-buffer request/response.

Добавить:

- request body chunks;
- response body chunks;
- bounded queues;
- backpressure;
- max frame size.

Test:

```text
100 MB upload
100 MB download
```

при этом память gateway не должна расти приблизительно на размер всего body.

---

# 79. Phase 9 — Cancellation and timeout

Добавить:

- public client disconnect;
- CANCEL_REQUEST;
- agent context cancellation;
- gateway timeout;
- local connect timeout;
- local read timeout.

Tests:

- slow local endpoint;
- disconnect browser;
- agent disconnect during request.

---

# 80. Phase 10 — Reconnect stability

Добавить:

- exponential backoff;
- jitter;
- automatic tunnel reopen;
- offline/online DB state;
- stale session cleanup.

Tests:

```text
kill network
restore network
gateway restart
agent restart
```

---

# 81. Phase 11 — Observability

Добавить:

- structured logs;
- request IDs;
- metrics;
- agent session metrics;
- tunnel request metrics;
- connection durations.

---

# 82. Phase 12 — Security hardening

Проверить:

- TLS;
- secrets;
- token hashing;
- rate limiting;
- CORS;
- secure cookies;
- body limits;
- header limits;
- WebSocket frame limits;
- auth brute force protection;
- revoked token behavior;
- session cleanup;
- permission checks.

---

# 83. Phase 13 — Production packaging

Backend:

```text
Dockerfile
```

Agent:

```text
linux amd64
linux arm64
windows amd64
darwin amd64
darwin arm64
```

Frontend:

```text
static build
```

Deployment:

```text
Caddy/Nginx
backend
postgres
```

---

# 84. Docker Compose

Target:

```yaml
services:
  postgres:
  backend:
  frontend:
  caddy:
```

Можно позже объединить frontend static build с Caddy.

---

# 85. Development environment

Локально:

```text
gateway.localhost
*.tunnel.localhost
```

Но wildcard localhost DNS behavior может отличаться.

Самый простой dev режим:

```text
public tunnel URL:
http://localhost:8080/t/{subdomain}/...
```

Production mode:

```text
https://{subdomain}.tunnel.example.com
```

Важно:

не смешивать это в domain logic.

Сделать HostResolver interface.

---

# 86. Testing strategy

## 86.1. Unit tests

Backend:

- auth;
- token generation;
- repositories;
- subdomain generator;
- host parser;
- protocol validation;
- error mapping.

Agent:

- URL construction;
- header filtering;
- reconnect backoff;
- protocol decoding.

---

## 86.2. Integration tests

PostgreSQL:

- repositories;
- migrations;
- unique constraints.

Gateway + fake agent:

- connect;
- open tunnel;
- forward request.

Agent + local httptest server:

- proxy request;
- body;
- headers;
- status.

---

# 87. End-to-end tests

Поднимать:

```text
Postgres
Backend/Gateway
Agent
Local test server
```

Проверять:

```text
public request -> gateway -> agent -> local server -> response
```

Scenarios:

- GET;
- POST JSON;
- file upload;
- 404 local;
- 500 local;
- timeout;
- agent offline;
- concurrent requests;
- token revoked.

---

# 88. Load tests

Минимальные сценарии:

```text
1 tunnel / 1 request
1 tunnel / 100 concurrent
10 tunnels / 10 concurrent each
large download
large upload
slow stream
```

Измерять:

```text
p50
p95
p99
error rate
memory
goroutines
connections
```

---

# 89. Coding standards

Go:

- `gofmt`;
- `go vet`;
- staticcheck;
- errors wrapped через `%w`;
- no global mutable state;
- context propagation;
- dependency injection через constructors;
- interfaces только в местах реальной abstraction boundary.

---

# 90. API versioning

REST:

```text
/api/v1
```

Tunnel protocol:

```text
protocolVersion: 1
```

Это два независимых version number.

---

# 91. Agent compatibility

Gateway должен при handshake проверить:

```text
protocolVersion
```

Если unsupported:

```text
PROTOCOL_VERSION_UNSUPPORTED
```

Позже можно поддерживать:

```text
v1
v2
```

одновременно.

---

# 92. Frontend state

Pinia stores:

```text
authStore
agentsStore
tunnelsStore
tokensStore
```

Не держать server state бесконтрольно во многих components.

---

# 93. Frontend polling

MVP:

```text
agents refresh каждые 10 секунд
tunnels refresh каждые 10 секунд
```

Позже заменить на SSE/WebSocket control updates.

---

# 94. User-facing CLI output

Пример:

```text
Tunnel Agent 0.1.0

Status      online
Agent       genmachine-pc
Forwarding  https://f7k2m1qa.tunnel.example.com
Local       http://127.0.0.1:8080

Requests
--------
GET  /health       200   12ms
POST /api/login    401   31ms
```

Для MVP request log можно сделать только в terminal.

---

# 95. CLI flags

```text
--token
--gateway
--local-host
--local-port
--log-level
--name
--insecure-dev
```

`--insecure-dev` только development.

Production по умолчанию не должен отключать TLS verification.

---

# 96. Configuration precedence

Порядок:

```text
CLI flags
environment
config file
defaults
```

Документировать.

---

# 97. Status lifecycle

Agent:

```text
connecting
online
reconnecting
offline
```

Tunnel:

```text
opening
online
closing
offline
```

Request:

```text
created
sending
processing
responding
completed
failed
cancelled
```

---

# 98. Data consistency

DB — источник metadata.

In-memory registry — источник active runtime state.

При server restart:

- active sessions пропадают;
- DB tunnels должны быть помечены offline;
- agent reconnect;
- runtime registry rebuild через reconnect.

При startup backend может выполнить:

```text
UPDATE agents SET status='offline' WHERE status='online';
UPDATE tunnels SET status='offline' WHERE status='online';
```

или использовать session TTL.

---

# 99. Multiple server instances — future

MVP:

```text
single gateway
```

Не строить distributed registry заранее.

Но abstraction должна позволить заменить:

```text
in-memory registry
```

на:

```text
Redis / message broker / dedicated gateway routing
```

позже.

---

# 100. Future architecture

Позже:

```text
                        Control Plane
                             |
                         PostgreSQL
                             |
              ┌──────────────┴──────────────┐
              |                             |
          Gateway 1                     Gateway 2
              ^                             ^
              |                             |
           Agents                        Agents
```

Control plane:

- users;
- tokens;
- desired state.

Gateway:

- active sessions;
- traffic.

---

# 101. Persistent tunnel names — future

MVP:

```text
random subdomain per session
```

Future:

```text
my-api.tunnel.example.com
```

Нужно:

```text
reserved_domains
user ownership
unique constraint
validation
```

---

# 102. WebSocket passthrough — future

Public application может само использовать WebSocket.

Тогда Gateway должен:

```text
HTTP Upgrade
```

передавать через tunnel как bidirectional byte stream.

Это отдельный protocol mode.

Не делать внутри первого HTTP implementation случайно и частично.

---

# 103. Raw TCP — future

Для TCP архитектура меняется:

```text
public TCP listener
      |
logical stream
      |
agent
      |
local TCP socket
```

Понадобится stream multiplexing.

Для этого можно позднее перейти на:

- yamux;
- smux;
- QUIC;
- HTTP/2 streams;
- custom stream protocol.

Но HTTP MVP не должен зависеть от этого.

---

# 104. Основные интерфейсы backend

Пример:

```go
type TokenAuthenticator interface {
    AuthenticateAgentToken(ctx context.Context, raw string) (*AgentIdentity, error)
}

type TunnelRegistry interface {
    Register(ctx context.Context, tunnel *ActiveTunnel) error
    Unregister(ctx context.Context, tunnelID uuid.UUID)
    FindByHostname(host string) (*ActiveTunnel, bool)
}

type RequestDispatcher interface {
    ForwardHTTP(w http.ResponseWriter, r *http.Request, tunnel *ActiveTunnel)
}
```

---

# 105. Основные интерфейсы agent

```go
type GatewayClient interface {
    Connect(ctx context.Context) error
    OpenTunnel(ctx context.Context, cfg TunnelConfig) (*TunnelInfo, error)
    Run(ctx context.Context) error
}

type LocalForwarder interface {
    Handle(ctx context.Context, req *RemoteHTTPRequest) (*RemoteHTTPResponse, error)
}
```

---

# 106. Definition of MVP complete

MVP считается законченным, если выполняется следующий сценарий.

## Setup

На сервере:

```text
PostgreSQL
Backend/Gateway
TLS
Wildcard DNS
Vue frontend
```

Пользователь:

1. открывает frontend;
2. регистрируется;
3. логинится;
4. создает agent token;
5. копирует token.

На локальном ПК:

```bash
tunnel-agent http 8080 --token tkn_xxxxx
```

Agent выводит:

```text
Connected
Forwarding https://abc123.tunnel.example.com -> http://127.0.0.1:8080
```

Пользователь запускает:

```bash
curl https://abc123.tunnel.example.com/api/health
```

Gateway:

- принимает request;
- находит tunnel;
- отправляет request agent;
- agent делает request localhost:8080;
- response возвращается наружу.

Поддерживаются параллельные requests.

Agent reconnect работает.

Revoked token больше не подключается.

---

# 107. Предлагаемый порядок задач для Codex

Ниже — последовательность, в которой лучше отдавать задачи Codex.

## Epic 1 — Repository Bootstrap

### Task 1.1
[x] Создать monorepo structure.

### Task 1.2
[x] Создать backend Go application bootstrap.

### Task 1.3
[x] Создать PostgreSQL config и connection pool.

### Task 1.4
[x] Добавить migrations.

### Task 1.5
[x] Добавить health endpoints.

### Task 1.6
[x] Создать Vue project.

### Task 1.7
[x] Создать agent Go module.

---

# 108. Epic 2 — Auth

### Task 2.1
[x] Создать `users` migration.

### Task 2.2
[x] Создать User model/repository.

### Task 2.3
[x] Реализовать password hashing.

### Task 2.4
[x] Реализовать register endpoint.

### Task 2.5
[x] Реализовать login endpoint.

### Task 2.6
[x] Реализовать access JWT.

### Task 2.7
[x] Реализовать refresh tokens.

### Task 2.8
[x] Реализовать logout.

### Task 2.9
[x] Реализовать `/me`.

### Task 2.10
[x] Frontend login/register.

---

# 109. Epic 3 — Agent Tokens

### Task 3.1
[x] Migration `agent_tokens`.

### Task 3.2
[x] Secure token generator.

### Task 3.3
[x] Token hashing.

### Task 3.4
[x] Create agent token endpoint.

### Task 3.5
[x] List agent tokens.

### Task 3.6
[x] Revoke agent token.

### Task 3.7
[x] Vue tokens page.

---

# 110. Epic 4 — Agent Connectivity

### Task 4.1
[x] Protocol package v1.

### Task 4.2
[x] WebSocket endpoint.

### Task 4.3
[x] Agent token authentication.

### Task 4.4
[x] ClientHello/ServerHello.

### Task 4.5
[x] SessionManager.

### Task 4.6
[x] Heartbeat.

### Task 4.7
[x] Agent CLI connection.

### Task 4.8
[x] Reconnect logic.

---

# 111. Epic 5 — Agents

### Task 5.1
[x] Migration `agents`.

### Task 5.2
[x] Register agent instance.

### Task 5.3
[x] Update connected state.

### Task 5.4
[x] Update last_seen.

### Task 5.5
[x] Mark disconnected.

### Task 5.6
[x] Agents REST endpoints.

### Task 5.7
[x] Vue agents page.

---

# 112. Epic 6 — Tunnels

### Task 6.1
[x] Migration `tunnels`.

### Task 6.2
[x] Subdomain generator.

### Task 6.3
[x] OpenTunnel protocol message.

### Task 6.4
[x] Tunnel registry.

### Task 6.5
[x] TunnelOpened response.

### Task 6.6
[x] CloseTunnel.

### Task 6.7
[x] Tunnel REST list.

### Task 6.8
[x] Vue tunnels page.

---

# 113. Epic 7 — HTTP Proxy Vertical Slice

### Task 7.1
[x] Public host parser.

### Task 7.2
[x] Public HTTP handler.

### Task 7.3
[x] Single request dispatch.

### Task 7.4
[x] Agent request receive.

### Task 7.5
[x] Local HTTP request.

### Task 7.6
[x] Response return.

### Task 7.7
[x] End-to-end GET test.

---

# 114. Epic 8 — Complete HTTP Forwarding

### Task 8.1
[x] Headers forwarding.

### Task 8.2
[x] Request body.

### Task 8.3
[x] Response body.

### Task 8.4
[x] All common HTTP methods.

### Task 8.5
[x] Status forwarding.

### Task 8.6
[x] Query strings.

### Task 8.7
[x] Hop-by-hop header filtering.

### Task 8.8
[x] Forwarded headers.

---

# 115. Epic 9 — Multiplexing

### Task 9.1
[x] Request IDs.

### Task 9.2
[x] Pending requests registry.

### Task 9.3
[x] Concurrent dispatch.

### Task 9.4
[x] Agent concurrent execution.

### Task 9.5
[x] Semaphore limits.

### Task 9.6
[x] Concurrency tests.

---

# 116. Epic 10 — Streaming

### Task 10.1
[x] Binary frame protocol.

### Task 10.2
[x] Request body chunks.

### Task 10.3
[x] Response body chunks.

### Task 10.4
[x] Bounded queues.

### Task 10.5
[x] Backpressure.

### Task 10.6
[x] Large body tests.

---

# 117. Epic 11 — Reliability

### Task 11.1
[x] Request timeouts.

### Task 11.2
[x] Cancellation.

### Task 11.3
[x] Agent disconnect cleanup.

### Task 11.4
[x] Tunnel state cleanup.

### Task 11.5
[x] Reconnect tunnel reopen.

### Task 11.6
[x] Graceful server shutdown.

### Task 11.7
[x] Graceful agent shutdown.

---

# 118. Epic 12 — Security

### Task 12.1
[x] Rate limiting auth.

### Task 12.2
[x] Rate limiting public requests.

### Task 12.3
[x] Body size limits.

### Task 12.4
[x] Frame size limits.

### Task 12.5
[x] Secure cookies.

### Task 12.6
[x] CORS configuration.

### Task 12.7
[x] Security headers.

### Task 12.8
[x] Revoked session disconnect.

---

# 119. Epic 13 — Observability

### Task 13.1
[x] Structured logger.

### Task 13.2
[x] Request ID middleware.

### Task 13.3
[x] Metrics endpoint.

### Task 13.4
[x] Gateway metrics.

### Task 13.5
[x] Agent session metrics.

### Task 13.6
[x] Tunnel traffic metrics.

---

# 120. Epic 14 — Deployment

### Task 14.1
[x] Backend Dockerfile.

### Task 14.2
[x] Frontend Dockerfile/static build.

### Task 14.3
[x] Docker Compose.

### Task 14.4
[x] Caddy/Nginx TLS.

### Task 14.5
[x] Wildcard DNS documentation.

### Task 14.6
[x] Agent release builds.

### Task 14.7
[x] systemd service example.

---

# Deferred backlog — Post-MVP (not included in the 102-task completion count)

### Persistent traffic history for dashboard

[ ] Persist tunnel traffic aggregates in PostgreSQL for historical dashboard
charts. Store time-bucketed request count, request bytes, response bytes, active
tunnels, and active agent connections; retain the existing in-memory counters
for live metrics. Add an authenticated dashboard API with user-scoped resource
counts and a retention/rollup policy before implementing charts. This is deferred
until after MVP deployment because it adds write load, data-retention decisions,
and migrations.

---

# 121. Codex task format

Каждую задачу лучше давать Codex в таком формате:

```text
Goal
Context
Files to inspect
Requirements
Non-goals
Acceptance criteria
Tests
Constraints
```

Пример:

```text
Goal:
Implement secure agent token creation.

Context:
Go backend with PostgreSQL.
Agent tokens authenticate tunnel-agent connections.

Requirements:
- Generate at least 256 bits of random entropy.
- Prefix token with "tkn_".
- Return plaintext token exactly once.
- Store SHA-256 hash only.
- Store visible prefix for UI identification.
- Add POST /api/v1/agent-tokens.
- Add unit and integration tests.

Non-goals:
- Agent WebSocket authentication.
- Token expiration UI.

Acceptance criteria:
- Endpoint requires authenticated user.
- Raw token is absent from PostgreSQL.
- Duplicate token collision is handled.
- Tests pass.
```

---

# 122. Правила работы Codex с проектом

Codex должен:

- сначала читать существующую architecture;
- автономно переходить к следующей задаче после успешной проверки текущей;
- запрашивать решение пользователя только при внешней блокировке, рисковом
  необратимом действии или неразрешимой неоднозначности требований;
- не менять API contracts без причины;
- не создавать новые frameworks без необходимости;
- не добавлять dependency, если standard library достаточно;
- писать tests вместе с feature;
- не смешивать несколько epic в один огромный change;
- сохранять backward compatibility protocol v1;
- запускать formatter/tests;
- обновлять docs при изменении protocol/API.

---

# 123. Definition of Done для каждой backend task

Каждая backend задача:

- код скомпилирован;
- unit tests;
- integration test при DB behavior;
- errors mapped;
- logging;
- context handling;
- migrations если schema changed;
- API docs updated;
- security implications checked.
- новые или изменённые unit/integration/API tests проходят.

---

# 124. Definition of Done для каждой agent task

- работает Linux;
- context cancellation;
- reconnect behavior не сломан;
- secrets не логируются;
- tests;
- CLI help updated;
- protocol compatibility preserved.
- новые или изменённые unit/protocol/local-proxy tests проходят.

---

# 125. Definition of Done для frontend task

- route/component;
- loading state;
- error state;
- empty state;
- API typing;
- unauthorized handling;
- responsive basic layout;
- no secrets persisted improperly.
- новые или изменённые component/store/API tests проходят.

---

# 126. Главные технические риски

## Risk 1 — multiplexing

Сложность:

```text
правильно сопоставлять concurrent requests/responses
```

Решение:

```text
requestId
pending request registry
single writer loop
```

---

## Risk 2 — memory pressure

Слабое решение:

```text
читать body целиком в []byte
```

Работает только для первого prototype.

Production решение:

```text
streaming chunks + bounded buffers
```

---

## Risk 3 — disconnected agent

Gateway не должен зависать.

Нужно:

```text
session context cancellation
cleanup pending requests
offline state
```

---

## Risk 4 — slow consumer

Если browser медленно читает response:

```text
gateway buffer может заполниться
```

Нужен backpressure.

---

## Risk 5 — slow local service

Agent должен иметь timeout и cancellation.

---

## Risk 6 — token leakage

Не логировать Authorization.

Не хранить plaintext token.

---

# 127. Архитектурные решения MVP

Факт реализации проекта должен опираться на следующие фиксированные решения:

```text
Backend language        Go
Agent language          Go
Frontend                Vue + TypeScript
Database                PostgreSQL
Agent transport         WebSocket over TLS
Public tunnel protocol  HTTP
Gateway model           single gateway instance
Tunnel routing          wildcard subdomain + Host header
Agent auth              opaque agent token
Web auth                JWT access + refresh token
Runtime registry        in-memory
Deployment              Docker Compose first
```

---

# 128. Что не следует усложнять на старте

Не использовать без необходимости:

```text
Kafka
RabbitMQ
Redis
Kubernetes
service mesh
microservices
event sourcing
CQRS
GraphQL
gRPC
distributed locks
multi-region
```

Для MVP это увеличит стоимость системы, но не решит core problem.

Core problem:

```text
secure persistent agent connection
+
request multiplexing
+
HTTP streaming
+
routing
```

---

# 129. Первый реально полезный milestone

Первый milestone должен закончиться не красивым dashboard, а работающим трафиком:

```text
local server
   ^
   |
agent
   ^
   |
gateway
   ^
   |
curl
```

То есть приоритет:

```text
auth
agent token
agent connection
open tunnel
GET forwarding
```

после этого расширять UI.

---

# 130. Рекомендуемый порядок приоритета

```text
P0
- backend bootstrap
- postgres
- user auth
- agent token
- websocket agent auth
- session manager
- open tunnel
- host routing
- basic HTTP forwarding

P1
- multiplexing
- reconnect
- full headers/body
- frontend dashboard
- token management
- agents/tunnels pages

P2
- streaming
- cancellation
- observability
- rate limiting
- production deployment

P3
- custom domain
- websocket passthrough
- raw TCP
- multi-gateway
```

---

# 131. Итоговая архитектура MVP

```text
                          ┌───────────────────────┐
                          │      Vue Frontend     │
                          └───────────┬───────────┘
                                      │
                                      │ HTTPS REST
                                      v
                    ┌─────────────────────────────────┐
                    │          Go Backend             │
                    │                                 │
                    │ Auth                            │
                    │ Users                           │
                    │ Agent Tokens                    │
                    │ Agents                          │
                    │ Tunnels                         │
                    │                                 │
                    │ Gateway                         │
                    │ ├─ Agent WebSocket Endpoint     │
                    │ ├─ Session Manager              │
                    │ ├─ Tunnel Registry              │
                    │ ├─ Request Dispatcher           │
                    │ └─ Public HTTP Handler          │
                    └────────────┬───────────┬────────┘
                                 │           │
                                 │           │
                                 v           │ WSS
                          ┌────────────┐      │
                          │ PostgreSQL │      │
                          └────────────┘      │
                                              v
                                  ┌────────────────────┐
                                  │      Go Agent      │
                                  │                    │
                                  │ Gateway Client     │
                                  │ Tunnel Manager     │
                                  │ Local HTTP Proxy   │
                                  └──────────┬─────────┘
                                             │
                                             v
                                   http://127.0.0.1:8080
```

---

# 132. Финальный критерий архитектуры

Система спроектирована правильно, если:

1. Локальный компьютер не требует public IP.
2. Локальный router не требует port forwarding.
3. Agent всегда инициирует соединение наружу.
4. Gateway никогда не пытается создать входящее соединение к локальному компьютеру.
5. Один agent connection может обслуживать много concurrent HTTP requests.
6. Request body и response body могут stream-иться.
7. Reconnect не требует ручной настройки gateway.
8. Revoked token немедленно или максимально быстро перестает давать доступ.
9. Gateway можно позже вынести в отдельный сервис.
10. HTTP tunneling можно позже расширить TCP tunneling без переписывания user/auth subsystem.
