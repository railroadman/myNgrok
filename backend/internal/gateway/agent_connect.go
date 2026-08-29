package gateway

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/myngrok/backend/internal/agents"
	"github.com/myngrok/backend/internal/protocol"
)

const defaultHeartbeatInterval = 20 * time.Second

const (
	sessionOutboundQueueCapacity = 32
	maxPendingResponseStreams    = 32
	websocketReadLimit           = 64 << 10
	maxTunnelResponseBodyBytes   = 32 << 20
)

type AgentConnectHandler struct {
	pool              *pgxpool.Pool
	agents            *agents.Service
	sessions          *SessionManager
	cleanupSession    func(context.Context, string, string)
	openTunnel        func(context.Context, string, string, string) (protocol.TunnelOpenedPayload, error)
	heartbeatInterval time.Duration
	validateToken     func(context.Context, string) bool
}

func NewAgentConnectHandler(pool *pgxpool.Pool, sessions *SessionManager, agentService *agents.Service, cleanupSession func(context.Context, string, string), openTunnel func(context.Context, string, string, string) (protocol.TunnelOpenedPayload, error)) *AgentConnectHandler {
	handler := &AgentConnectHandler{pool: pool, agents: agentService, sessions: sessions, cleanupSession: cleanupSession, openTunnel: openTunnel, heartbeatInterval: defaultHeartbeatInterval}
	handler.validateToken = handler.valid
	return handler
}

func (h *AgentConnectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rawToken := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if !strings.HasPrefix(rawToken, "tkn_") || !h.validateToken(r.Context(), rawToken) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(websocketReadLimit)
	defer conn.CloseNow()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	if !h.handshake(ctx, conn, rawToken) {
		return
	}
}

func (h *AgentConnectHandler) handshake(ctx context.Context, conn *websocket.Conn, rawToken string) bool {
	_, data, err := conn.Read(ctx)
	if err != nil {
		return false
	}
	var hello struct {
		Type    string                      `json:"type"`
		Payload protocol.ClientHelloPayload `json:"payload"`
	}
	if json.Unmarshal(data, &hello) != nil || hello.Type != protocol.ClientHello || hello.Payload.ProtocolVersion != protocol.Version {
		_ = conn.Close(websocket.StatusPolicyViolation, "unsupported protocol")
		return false
	}

	sessionID, err := newSessionID()
	if err != nil {
		_ = conn.Close(websocket.StatusInternalError, "could not create session")
		return false
	}
	agentID := ""
	if h.agents != nil {
		agentID, err = h.agents.Connect(ctx, rawToken, hello.Payload)
		if err != nil {
			_ = conn.Close(websocket.StatusPolicyViolation, "agent registration failed")
			return false
		}
	}
	sessionCtx, sessionCancel := context.WithCancel(ctx)
	tokenSum := sha256.Sum256([]byte(rawToken))
	h.sessions.Register(Session{ID: sessionID, AgentID: agentID, AgentTokenHash: hex.EncodeToString(tokenSum[:]), Outbound: make(chan OutboundMessage, sessionOutboundQueueCapacity), Cancel: sessionCancel})
	defer func() {
		sessionCancel()
		h.sessions.Remove(sessionID)
		if h.cleanupSession != nil {
			h.cleanupSession(context.Background(), sessionID, agentID)
		}
		if h.agents != nil && agentID != "" {
			h.agents.Disconnect(context.Background(), agentID)
		}
	}()

	response, _ := json.Marshal(protocol.Envelope{Type: protocol.ServerHello, Payload: protocol.ServerHelloPayload{
		ProtocolVersion: protocol.Version, SessionID: sessionID, HeartbeatIntervalSeconds: int(h.heartbeatInterval.Seconds()),
	}})
	if err := conn.Write(ctx, websocket.MessageText, response); err != nil {
		return false
	}
	return h.serveSession(sessionCtx, conn, sessionID)
}

func (h *AgentConnectHandler) serveSession(ctx context.Context, conn *websocket.Conn, sessionID string) bool {
	ticker := time.NewTicker(h.heartbeatInterval)
	defer ticker.Stop()
	type inboundMessage struct {
		messageType websocket.MessageType
		data        []byte
	}
	type responseAssembly struct {
		payload protocol.HTTPResponsePayload
		next    uint32
		body    []byte
	}
	responses := make(map[string]*responseAssembly)
	readCh := make(chan inboundMessage, 1)
	errCh := make(chan error, 1)
	go func() {
		for {
			messageType, data, err := conn.Read(ctx)
			if err != nil {
				errCh <- err
				return
			}
			select {
			case readCh <- inboundMessage{messageType: messageType, data: data}:
			case <-ctx.Done():
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return true
		case <-ticker.C:
			ping, _ := json.Marshal(protocol.Envelope{Type: protocol.Ping})
			if err := conn.Write(ctx, websocket.MessageText, ping); err != nil {
				return false
			}
		case message := <-h.outbound(sessionID):
			if err := conn.Write(ctx, message.Type, message.Data); err != nil {
				return false
			}
		case inbound := <-readCh:
			if inbound.messageType == websocket.MessageBinary {
				frame, err := protocol.ParseBinaryFrame(inbound.data)
				assembly, ok := responses[frame.RequestID]
				if err != nil || !ok || frame.Type != protocol.ResponseBodyChunk || frame.Sequence != assembly.next || int64(len(assembly.body)+len(frame.Payload)) > maxTunnelResponseBodyBytes || (assembly.payload.ContentLength >= 0 && int64(len(assembly.body)+len(frame.Payload)) > assembly.payload.ContentLength) {
					_ = conn.Close(websocket.StatusUnsupportedData, "invalid response body chunk")
					return false
				}
				assembly.body = append(assembly.body, frame.Payload...)
				assembly.next++
				continue
			}
			if inbound.messageType != websocket.MessageText {
				_ = conn.Close(websocket.StatusUnsupportedData, "invalid message type")
				return false
			}
			var message struct {
				Type    string          `json:"type"`
				Payload json.RawMessage `json:"payload"`
			}
			if json.Unmarshal(inbound.data, &message) != nil {
				_ = conn.Close(websocket.StatusUnsupportedData, "invalid message")
				return false
			}
			switch message.Type {
			case protocol.HTTPResponseStart:
				var payload protocol.HTTPResponsePayload
				if json.Unmarshal(message.Payload, &payload) != nil || payload.RequestID == "" || payload.ContentLength < -1 || payload.ContentLength > maxTunnelResponseBodyBytes {
					_ = conn.Close(websocket.StatusUnsupportedData, "invalid response start")
					return false
				}
				if _, exists := responses[payload.RequestID]; !exists && len(responses) >= maxPendingResponseStreams {
					_ = conn.Close(websocket.StatusPolicyViolation, "too many response streams")
					return false
				}
				responses[payload.RequestID] = &responseAssembly{payload: payload}
			case protocol.OpenTunnel:
				var payload protocol.OpenTunnelPayload
				if h.openTunnel == nil || json.Unmarshal(message.Payload, &payload) != nil || payload.RequestID == "" || strings.TrimSpace(payload.LocalAddress) == "" {
					return false
				}
				agentID, ok := h.sessions.AgentID(sessionID)
				if !ok {
					return false
				}
				opened, err := h.openTunnel(ctx, sessionID, agentID, payload.LocalAddress)
				if err != nil {
					return false
				}
				opened.RequestID = payload.RequestID
				encoded, _ := json.Marshal(protocol.Envelope{Type: protocol.TunnelOpened, Payload: opened})
				if !h.sessions.Send(sessionID, encoded) {
					return false
				}
			case protocol.HTTPResponseEnd:
				var end struct {
					Payload protocol.HTTPResponseEndPayload `json:"payload"`
				}
				if json.Unmarshal(inbound.data, &end) != nil {
					return false
				}
				assembly, ok := responses[end.Payload.RequestID]
				if !ok || (assembly.payload.ContentLength >= 0 && int64(len(assembly.body)) != assembly.payload.ContentLength) {
					return false
				}
				delete(responses, end.Payload.RequestID)
				h.sessions.DeliverResponse(end.Payload.RequestID, Response{StatusCode: assembly.payload.StatusCode, Headers: assembly.payload.Headers, Body: assembly.body, Error: assembly.payload.Error})
			case protocol.Pong:
				h.sessions.Touch(sessionID)
				h.touchAgent(sessionID)
			case protocol.Ping:
				pong, _ := json.Marshal(protocol.Envelope{Type: protocol.Pong})
				if err := conn.Write(ctx, websocket.MessageText, pong); err != nil {
					return false
				}
				h.sessions.Touch(sessionID)
				h.touchAgent(sessionID)
			}
		case <-errCh:
			return true
		}
	}
}

func (h *AgentConnectHandler) outbound(sessionID string) <-chan OutboundMessage {
	// The session is registered before serveSession is entered and removed only
	// after it returns. A small fallback channel prevents a nil select case.
	h.sessions.mu.RLock()
	session := h.sessions.sessions[sessionID]
	h.sessions.mu.RUnlock()
	return session.Outbound
}

func (h *AgentConnectHandler) touchAgent(sessionID string) {
	if h.agents == nil {
		return
	}
	if agentID, ok := h.sessions.AgentID(sessionID); ok && agentID != "" {
		h.agents.Touch(context.Background(), agentID)
	}
}

func (h *AgentConnectHandler) valid(ctx context.Context, rawToken string) bool {
	sum := sha256.Sum256([]byte(rawToken))
	var ok bool
	_ = h.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM agent_tokens WHERE token_hash=$1 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at>NOW()))`, hex.EncodeToString(sum[:])).Scan(&ok)
	return ok
}

func newSessionID() (string, error) {
	bytes := make([]byte, 18)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "ses_" + base64.RawURLEncoding.EncodeToString(bytes), nil
}
