# Tunnel protocol v1

Control messages use JSON WebSocket text frames. Request metadata is sent as
`http_request_start`, followed by binary request-body chunks and an
`http_request_end` control message. Response metadata follows the matching
`http_response_start` → binary chunks → `http_response_end` sequence.

`http_request_start` contains `requestId`, `method`, `path`, `headers`, and
`contentLength` (or `-1` when it is unknown). Headers use the JSON shape
`map[string][]string` so repeated HTTP fields retain their values.
`http_response_start` contains `requestId`, `statusCode`, `headers`, optional `error`,
and `contentLength` (currently `-1` when the local service does not announce it).
The gateway and agent copy header fields in both directions.

Each forwarded HTTP request has a cryptographically random `requestId`. The gateway
keeps an in-memory pending-request registry keyed by this value, so responses may
arrive in a different order without being delivered to the wrong public request.
The agent executes local requests concurrently, with a configurable per-connection
semaphore (default 16), and emits all resulting messages through one WebSocket writer.

`cancel_request` carries `requestId` and tells the agent to cancel the matching local
HTTP context when the public client disconnects or the gateway deadline expires.

Body chunks now have a versioned binary-frame contract: `version(1)`, `type(1)`,
`requestIdLength(2)`, `sequence(4)`, `payloadLength(4)`, `requestId`, `payload`.
Integer fields use big-endian byte order. Type `1` is a request-body chunk and type
`2` is a response-body chunk. The gateway reads public request bodies in 32 KiB
chunks, sends each as type `1` in sequence order, then sends `http_request_end`.
The agent validates request sequence ordering and the announced length before
forwarding the assembled request locally. It then reads the local response in 32 KiB
chunks and sends type `2` frames. The gateway validates their order and length before
completing the matching public request.

Each gateway session and agent connection has a bounded outbound frame queue (32
messages). The agent also accepts at most 32 incomplete request streams, and the
gateway at most 32 incomplete response streams per session. Queue overflow is
reported to the request path or closes a protocol-violating peer rather than allowing
unbounded memory growth. The agent dispatches local work through a queue bounded by
its concurrency setting (maximum 64), so requests waiting for a local worker do not
create unbounded goroutines.

When a gateway→agent queue is full, the gateway waits for capacity while it is
forwarding a public request body. It stops reading more bytes from the public client
until the WebSocket writer drains the queue; client cancellation interrupts that wait.
The agent applies the same waiting behavior to its outbound response queue. This
propagates slow-consumer pressure across the tunnel instead of accumulating data.

Regression coverage exercises multi-megabyte uploads and downloads, asserting chunk
sequence, maximum chunk size, and end-to-end SHA-256 integrity.

Before sending a public request to the agent, the gateway removes HTTP hop-by-hop
headers (`Connection`, `Proxy-Connection`, `Keep-Alive`, `Proxy-Authenticate`,
`Proxy-Authorization`, `TE`, `Trailer`, `Transfer-Encoding`, `Upgrade`) and any
header named by `Connection`. It does the same for local response headers. The agent
also performs this filtering before issuing the local request and before returning
the local response.

The gateway replaces untrusted incoming forwarded fields and supplies
`X-Forwarded-For`, `X-Forwarded-Host`, `X-Forwarded-Proto`, and
`X-Tunnel-Request-ID`. The local request host remains the configured local address.
