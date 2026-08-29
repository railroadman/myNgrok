package gateway

import (
	"context"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Session describes one currently connected agent. It is deliberately kept in
// memory: a disconnected agent must not appear as available for a tunnel.
type Session struct {
	ID             string
	AgentID        string
	AgentTokenHash string
	LastSeen       time.Time
	Outbound       chan OutboundMessage
	Cancel         context.CancelFunc
}

type OutboundMessage struct {
	Type websocket.MessageType
	Data []byte
}

func (m *SessionManager) Send(id string, message []byte) bool {
	m.mu.RLock()
	session, ok := m.sessions[id]
	m.mu.RUnlock()
	if !ok || session.Outbound == nil {
		return false
	}
	select {
	case session.Outbound <- OutboundMessage{Type: websocket.MessageText, Data: message}:
		return true
	default:
		return false
	}
}

func (m *SessionManager) SendBinary(id string, message []byte) bool {
	m.mu.RLock()
	session, ok := m.sessions[id]
	m.mu.RUnlock()
	if !ok || session.Outbound == nil {
		return false
	}
	select {
	case session.Outbound <- OutboundMessage{Type: websocket.MessageBinary, Data: message}:
		return true
	default:
		return false
	}
}

// SendContext waits for outbound queue capacity. It is used on the public
// request path so a slow agent propagates backpressure to the public client.
func (m *SessionManager) SendContext(ctx context.Context, id string, message []byte) bool {
	return m.sendContext(ctx, id, OutboundMessage{Type: websocket.MessageText, Data: message})
}

func (m *SessionManager) SendBinaryContext(ctx context.Context, id string, message []byte) bool {
	return m.sendContext(ctx, id, OutboundMessage{Type: websocket.MessageBinary, Data: message})
}

func (m *SessionManager) sendContext(ctx context.Context, id string, message OutboundMessage) bool {
	m.mu.RLock()
	session, ok := m.sessions[id]
	m.mu.RUnlock()
	if !ok || session.Outbound == nil {
		return false
	}
	select {
	case session.Outbound <- message:
		return true
	case <-ctx.Done():
		return false
	}
}

type SessionManager struct {
	mu                  sync.RWMutex
	sessions            map[string]Session
	pending             map[string]pendingRequest
	connectionsTotal    uint64
	disconnectionsTotal uint64
}

type pendingRequest struct {
	sessionID string
	response  chan Response
}
type Response struct {
	StatusCode int
	Headers    map[string][]string
	Body       []byte
	Error      string
}

func NewSessionManager() *SessionManager {
	return &SessionManager{sessions: make(map[string]Session), pending: make(map[string]pendingRequest)}
}
func (m *SessionManager) RegisterRequest(sessionID, id string) chan Response {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan Response, 1)
	m.pending[id] = pendingRequest{sessionID: sessionID, response: ch}
	return ch
}
func (m *SessionManager) CancelRequest(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.pending, id)
}
func (m *SessionManager) DeliverResponse(id string, response Response) bool {
	m.mu.Lock()
	pending, ok := m.pending[id]
	if ok {
		delete(m.pending, id)
	}
	m.mu.Unlock()
	if !ok {
		return false
	}
	pending.response <- response
	return true
}

func (m *SessionManager) Register(session Session) {
	if session.LastSeen.IsZero() {
		session.LastSeen = time.Now().UTC()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[session.ID] = session
	m.connectionsTotal++
}

func (m *SessionManager) Touch(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, ok := m.sessions[id]
	if ok {
		session.LastSeen = time.Now().UTC()
		m.sessions[id] = session
	}
	return ok
}

func (m *SessionManager) Remove(id string) {
	m.mu.Lock()
	if _, exists := m.sessions[id]; exists {
		delete(m.sessions, id)
		m.disconnectionsTotal++
	}
	var cancelled []chan Response
	for requestID, pending := range m.pending {
		if pending.sessionID == id {
			delete(m.pending, requestID)
			cancelled = append(cancelled, pending.response)
		}
	}
	m.mu.Unlock()
	for _, response := range cancelled {
		response <- Response{Error: "tunnel agent disconnected"}
	}
}

type MetricsSnapshot struct {
	ActiveSessions      uint64
	PendingRequests     uint64
	ConnectionsTotal    uint64
	DisconnectionsTotal uint64
}

func (m *SessionManager) MetricsSnapshot() MetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return MetricsSnapshot{ActiveSessions: uint64(len(m.sessions)), PendingRequests: uint64(len(m.pending)), ConnectionsTotal: m.connectionsTotal, DisconnectionsTotal: m.disconnectionsTotal}
}

// CloseAll requests a graceful close of every active agent WebSocket session.
// Each handler performs its normal disconnect cleanup before its socket closes.
func (m *SessionManager) CloseAll() {
	m.mu.RLock()
	cancels := make([]context.CancelFunc, 0, len(m.sessions))
	for _, session := range m.sessions {
		if session.Cancel != nil {
			cancels = append(cancels, session.Cancel)
		}
	}
	m.mu.RUnlock()
	for _, cancel := range cancels {
		cancel()
	}
}

// DisconnectTokenHash cancels every active session authenticated by a revoked
// agent token. Session handlers then perform their ordinary disconnect cleanup.
func (m *SessionManager) DisconnectTokenHash(tokenHash string) {
	m.mu.RLock()
	cancels := make([]context.CancelFunc, 0)
	for _, session := range m.sessions {
		if session.AgentTokenHash == tokenHash && session.Cancel != nil {
			cancels = append(cancels, session.Cancel)
		}
	}
	m.mu.RUnlock()
	for _, cancel := range cancels {
		cancel()
	}
}

func (m *SessionManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

func (m *SessionManager) AgentID(id string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, ok := m.sessions[id]
	return session.AgentID, ok
}
