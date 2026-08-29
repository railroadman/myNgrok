package gatewayclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const protocolVersion = 1

const (
	defaultMaxConcurrentRequests = 16
	maxConcurrentRequests        = 64
	maxPendingRequestStreams     = 32
	outboundQueueCapacity        = 32
	websocketReadLimit           = 64 << 10
)

type Config struct {
	GatewayURL            string
	Token                 string
	Version               string
	LocalAddress          string
	MaxConcurrentRequests int
	OnRequest             func(Request)
	OnSessionMetrics      func(SessionMetrics)
}

type Connected struct {
	SessionID string
}
type SessionMetrics struct {
	ActiveSessions      int
	ConnectionsTotal    uint64
	DisconnectionsTotal uint64
}
type Request struct {
	ID      string
	Method  string
	Path    string
	Headers http.Header
	Body    []byte
}

type Client struct {
	dial                  func(context.Context, string, *websocket.DialOptions) (*websocket.Conn, *http.Response, error)
	wait                  func(context.Context, time.Duration) error
	reconnectInitialDelay time.Duration
	reconnectMaxDelay     time.Duration
}

func New() *Client {
	return &Client{
		dial:                  websocket.Dial,
		wait:                  waitFor,
		reconnectInitialDelay: time.Second,
		reconnectMaxDelay:     30 * time.Second,
	}
}

// Run keeps the agent online. Each failed or closed session is retried with an
// exponential backoff capped at 30 seconds, until the caller cancels ctx.
func (c *Client) Run(ctx context.Context, config Config, connected func(Connected), retrying func(error, time.Duration)) error {
	delay := c.reconnectInitialDelay
	metrics := SessionMetrics{}
	for {
		connectedInSession := false
		err := c.Connect(ctx, config, func(connection Connected) {
			connectedInSession = true
			metrics.ActiveSessions = 1
			metrics.ConnectionsTotal++
			if config.OnSessionMetrics != nil {
				config.OnSessionMetrics(metrics)
			}
			if connected != nil {
				connected(connection)
			}
		})
		if connectedInSession {
			metrics.ActiveSessions = 0
			metrics.DisconnectionsTotal++
			if config.OnSessionMetrics != nil {
				config.OnSessionMetrics(metrics)
			}
		}
		if ctx.Err() != nil {
			return nil
		}
		if err == nil {
			delay = c.reconnectInitialDelay
			continue
		}
		if retrying != nil {
			retrying(err, delay)
		}
		if err := c.wait(ctx, delay); err != nil {
			return nil
		}
		delay *= 2
		if delay > c.reconnectMaxDelay {
			delay = c.reconnectMaxDelay
		}
	}
}

// Connect establishes one agent session and serves heartbeat messages until the
// gateway or caller closes the connection.
func (c *Client) Connect(ctx context.Context, config Config, connected func(Connected)) error {
	if config.GatewayURL == "" || config.Token == "" {
		return fmt.Errorf("gateway URL and token are required")
	}
	maxConcurrent := config.MaxConcurrentRequests
	if maxConcurrent <= 0 {
		maxConcurrent = defaultMaxConcurrentRequests
	}
	if maxConcurrent > maxConcurrentRequests {
		return fmt.Errorf("max concurrent requests exceeds %d", maxConcurrentRequests)
	}
	conn, _, err := c.dial(ctx, config.GatewayURL, &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": {"Bearer " + config.Token}}})
	if err != nil {
		return fmt.Errorf("connect to gateway: %w", err)
	}
	defer conn.CloseNow()
	conn.SetReadLimit(websocketReadLimit)

	hostname, _ := os.Hostname()
	hello, _ := json.Marshal(envelope{Type: "client_hello", Payload: clientHello{
		ProtocolVersion: protocolVersion,
		AgentVersion:    config.Version,
		InstanceID:      hostname,
		Hostname:        hostname,
		OS:              runtime.GOOS,
		Arch:            runtime.GOARCH,
	}})
	if err := conn.Write(ctx, websocket.MessageText, hello); err != nil {
		return fmt.Errorf("send client hello: %w", err)
	}

	_, data, err := conn.Read(ctx)
	if err != nil {
		return fmt.Errorf("read server hello: %w", err)
	}
	var helloResponse struct {
		Type    string      `json:"type"`
		Payload serverHello `json:"payload"`
	}
	if err := json.Unmarshal(data, &helloResponse); err != nil || helloResponse.Type != "server_hello" || helloResponse.Payload.ProtocolVersion != protocolVersion || helloResponse.Payload.SessionID == "" {
		return fmt.Errorf("gateway returned an invalid server hello")
	}
	if connected != nil {
		connected(Connected{SessionID: helloResponse.Payload.SessionID})
	}
	sessionCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan func(), maxConcurrent)
	var activeMu sync.Mutex
	activeCancels := map[string]context.CancelFunc{}
	pendingRequests := map[string]struct {
		payload   httpRequestPayload
		assembler *chunkAssembler
	}{}
	type outboundMessage struct {
		messageType websocket.MessageType
		data        []byte
	}
	// Keep queued WebSocket frames bounded independently of request concurrency.
	// A full channel makes send wait for the writer or session cancellation.
	outbound := make(chan outboundMessage, outboundQueueCapacity)
	go func() {
		for {
			select {
			case <-sessionCtx.Done():
				return
			case message := <-outbound:
				if err := conn.Write(sessionCtx, message.messageType, message.data); err != nil {
					cancel()
					return
				}
			}
		}
	}()
	send := func(messageType websocket.MessageType, message []byte) bool {
		select {
		case outbound <- outboundMessage{messageType: messageType, data: message}:
			return true
		case <-sessionCtx.Done():
			return false
		}
	}
	if localAddress := strings.TrimSpace(config.LocalAddress); localAddress != "" {
		open, _ := json.Marshal(envelope{Type: openTunnelType, Payload: openTunnelPayload{RequestID: "open_" + helloResponse.Payload.SessionID, LocalAddress: localAddress}})
		if !send(websocket.MessageText, open) {
			return fmt.Errorf("gateway connection closed before tunnel open")
		}
	}
	for range maxConcurrent {
		go func() {
			for {
				select {
				case <-sessionCtx.Done():
					return
				case job := <-jobs:
					job()
				}
			}
		}()
	}

	for {
		messageType, data, err := conn.Read(sessionCtx)
		if err != nil {
			return fmt.Errorf("gateway connection closed: %w", err)
		}
		if messageType == websocket.MessageBinary {
			frame, frameErr := parseBinaryFrame(data)
			if frameErr != nil || frame.Type != requestBodyChunk {
				return fmt.Errorf("invalid request body chunk: %w", frameErr)
			}
			pending, ok := pendingRequests[frame.RequestID]
			if !ok {
				return fmt.Errorf("request body chunk for unknown request %q", frame.RequestID)
			}
			if _, frameErr = pending.assembler.add(frame); frameErr != nil {
				return frameErr
			}
			pendingRequests[frame.RequestID] = pending
			continue
		}
		if messageType != websocket.MessageText {
			return fmt.Errorf("unsupported gateway message type %d", messageType)
		}
		var message struct {
			Type    string             `json:"type"`
			Payload httpRequestPayload `json:"payload"`
		}
		if err := json.Unmarshal(data, &message); err != nil {
			return fmt.Errorf("invalid gateway message: %w", err)
		}
		if message.Type == "ping" {
			pong, _ := json.Marshal(envelope{Type: "pong"})
			send(websocket.MessageText, pong)
		}
		if message.Type == httpRequestStartType {
			if message.Payload.RequestID == "" {
				return fmt.Errorf("request start has no request ID")
			}
			if _, exists := pendingRequests[message.Payload.RequestID]; exists {
				return fmt.Errorf("duplicate request start %q", message.Payload.RequestID)
			}
			if len(pendingRequests) >= maxPendingRequestStreams {
				return fmt.Errorf("too many pending request streams")
			}
			assembler, assembleErr := newChunkAssembler(message.Payload.ContentLength)
			if assembleErr != nil {
				return assembleErr
			}
			pendingRequests[message.Payload.RequestID] = struct {
				payload   httpRequestPayload
				assembler *chunkAssembler
			}{payload: message.Payload, assembler: assembler}
		}
		if message.Type == httpRequestEndType {
			var end struct {
				Payload httpRequestEndPayload `json:"payload"`
			}
			if err := json.Unmarshal(data, &end); err != nil {
				return fmt.Errorf("invalid request end: %w", err)
			}
			pending, ok := pendingRequests[end.Payload.RequestID]
			if !ok {
				return fmt.Errorf("request end for unknown request %q", end.Payload.RequestID)
			}
			if err := pending.assembler.complete(); err != nil {
				return err
			}
			delete(pendingRequests, end.Payload.RequestID)
			request := Request{ID: pending.payload.RequestID, Method: pending.payload.Method, Path: pending.payload.Path, Headers: http.Header(pending.payload.Headers), Body: pending.assembler.body}
			requestCtx, requestCancel := context.WithCancel(sessionCtx)
			activeMu.Lock()
			activeCancels[request.ID] = requestCancel
			activeMu.Unlock()
			job := func() {
				defer func() { activeMu.Lock(); delete(activeCancels, request.ID); activeMu.Unlock(); requestCancel() }()
				if config.OnRequest != nil {
					config.OnRequest(request)
				}
				if config.LocalAddress != "" {
					sequence := uint32(0)
					responseStarted := false
					sendStart := func(local LocalResponse) error {
						encoded, _ := json.Marshal(envelope{Type: httpResponseStartType, Payload: httpResponsePayload{RequestID: request.ID, StatusCode: local.StatusCode, Headers: local.Headers, ContentLength: -1}})
						if !send(websocket.MessageText, encoded) {
							return fmt.Errorf("gateway connection closed")
						}
						responseStarted = true
						return nil
					}
					sendChunk := func(payload []byte) error {
						encoded, err := (binaryFrame{Type: responseBodyChunk, RequestID: request.ID, Sequence: sequence, Payload: payload}).marshalBinary()
						if err != nil {
							return err
						}
						if !send(websocket.MessageBinary, encoded) {
							return fmt.Errorf("gateway connection closed")
						}
						sequence++
						return nil
					}
					if localErr := StreamLocal(requestCtx, config.LocalAddress, request, sendStart, sendChunk); localErr != nil && !responseStarted {
						encoded, _ := json.Marshal(envelope{Type: httpResponseStartType, Payload: httpResponsePayload{RequestID: request.ID, StatusCode: 502, ContentLength: 0, Error: localErr.Error()}})
						send(websocket.MessageText, encoded)
					}
					encoded, _ := json.Marshal(envelope{Type: httpResponseEndType, Payload: httpResponseEndPayload{RequestID: request.ID}})
					send(websocket.MessageText, encoded)
				}
			}
			select {
			case jobs <- job:
			case <-sessionCtx.Done():
				return nil
			}
		}
		if message.Type == cancelRequestType {
			delete(pendingRequests, message.Payload.RequestID)
			activeMu.Lock()
			cancel := activeCancels[message.Payload.RequestID]
			activeMu.Unlock()
			if cancel != nil {
				cancel()
			}
		}
	}
}

type envelope struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

type clientHello struct {
	ProtocolVersion int    `json:"protocolVersion"`
	AgentVersion    string `json:"agentVersion"`
	InstanceID      string `json:"instanceId"`
	Hostname        string `json:"hostname"`
	OS              string `json:"os"`
	Arch            string `json:"arch"`
}

type serverHello struct {
	ProtocolVersion int    `json:"protocolVersion"`
	SessionID       string `json:"sessionId"`
}

func waitFor(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
