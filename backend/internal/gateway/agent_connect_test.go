package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/myngrok/backend/internal/agents"
	"github.com/myngrok/backend/internal/database"
	"github.com/myngrok/backend/internal/protocol"
)

func TestAgentConnectCompletesHandshakeAndHeartbeat(t *testing.T) {
	sessions := NewSessionManager()
	handler := &AgentConnectHandler{
		sessions:          sessions,
		heartbeatInterval: 25 * time.Millisecond,
		validateToken:     func(context.Context, string) bool { return true },
	}
	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, response, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": {"Bearer tkn_test"}}})
	if err != nil {
		t.Fatalf("Dial() error = %v (status %v)", err, response)
	}
	defer conn.CloseNow()

	hello, _ := json.Marshal(protocol.Envelope{Type: protocol.ClientHello, Payload: protocol.ClientHelloPayload{ProtocolVersion: protocol.Version}})
	if err := conn.Write(ctx, websocket.MessageText, hello); err != nil {
		t.Fatalf("write hello: %v", err)
	}
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read server hello: %v", err)
	}
	var serverHello struct {
		Type    string                      `json:"type"`
		Payload protocol.ServerHelloPayload `json:"payload"`
	}
	if err := json.Unmarshal(data, &serverHello); err != nil {
		t.Fatalf("decode server hello: %v", err)
	}
	if serverHello.Type != protocol.ServerHello || !strings.HasPrefix(serverHello.Payload.SessionID, "ses_") {
		t.Fatalf("unexpected server hello: %#v", serverHello)
	}
	if got := sessions.Count(); got != 1 {
		t.Fatalf("sessions.Count() = %d, want 1", got)
	}

	_, data, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("read ping: %v", err)
	}
	var ping struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &ping); err != nil || ping.Type != protocol.Ping {
		t.Fatalf("unexpected heartbeat: %s (%v)", data, err)
	}
	pong, _ := json.Marshal(protocol.Envelope{Type: protocol.Pong})
	if err := conn.Write(ctx, websocket.MessageText, pong); err != nil {
		t.Fatalf("write pong: %v", err)
	}
}

func TestNewAgentConnectHandlerValidatesStoredTokenAndTouchesAgent(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://tunnel:tunnel@127.0.0.1:15432/tunnel?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	email := fmt.Sprintf("gateway-handler-%d@example.test", time.Now().UnixNano())
	var userID string
	if err := pool.Raw().QueryRow(ctx, `INSERT INTO users (email,password_hash) VALUES ($1,'test') RETURNING id::text`, email).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	defer pool.Raw().Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID)
	rawToken := "tkn_gateway_handler"
	sum := sha256.Sum256([]byte(rawToken))
	if _, err := pool.Raw().Exec(ctx, `INSERT INTO agent_tokens (user_id,name,token_prefix,token_hash) VALUES ($1,'test','tkn_gateway_', $2)`, userID, hex.EncodeToString(sum[:])); err != nil {
		t.Fatal(err)
	}
	manager := NewSessionManager()
	service := agents.NewService(pool.Raw())
	handler := NewAgentConnectHandler(pool.Raw(), manager, service, nil, nil)
	if !handler.valid(ctx, rawToken) || handler.valid(ctx, "tkn_invalid") {
		t.Fatal("stored token validation is incorrect")
	}
	agentID, err := service.Connect(ctx, rawToken, protocol.ClientHelloPayload{InstanceID: "touch-instance", Hostname: "host", OS: "windows", Arch: "amd64", AgentVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	manager.Register(Session{ID: "ses_touch", AgentID: agentID})
	handler.touchAgent("ses_touch")
}

func TestAgentConnectRejectsInvalidTokenBeforeUpgrade(t *testing.T) {
	handler := &AgentConnectHandler{sessions: NewSessionManager(), validateToken: func(context.Context, string) bool { return false }}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/agent/connect", nil)
	request.Header.Set("Authorization", "Bearer tkn_invalid")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestAgentConnectRejectsUnsupportedProtocolHello(t *testing.T) {
	handler := &AgentConnectHandler{sessions: NewSessionManager(), heartbeatInterval: time.Hour, validateToken: func(context.Context, string) bool { return true }}
	server := httptest.NewServer(handler)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": {"Bearer tkn_test"}}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	invalid, _ := json.Marshal(protocol.Envelope{Type: protocol.ClientHello, Payload: protocol.ClientHelloPayload{ProtocolVersion: protocol.Version + 1}})
	if err := conn.Write(ctx, websocket.MessageText, invalid); err != nil {
		t.Fatal(err)
	}
	if _, _, err := conn.Read(ctx); err == nil {
		t.Fatal("unsupported hello remained connected")
	}
}

func TestAgentConnectRespondsToAgentPingAndRejectsInvalidResponseStart(t *testing.T) {
	handler := &AgentConnectHandler{sessions: NewSessionManager(), heartbeatInterval: time.Hour, validateToken: func(context.Context, string) bool { return true }}
	server := httptest.NewServer(handler)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": {"Bearer tkn_test"}}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	hello, _ := json.Marshal(protocol.Envelope{Type: protocol.ClientHello, Payload: protocol.ClientHelloPayload{ProtocolVersion: protocol.Version}})
	if err := conn.Write(ctx, websocket.MessageText, hello); err != nil {
		t.Fatal(err)
	}
	if _, _, err := conn.Read(ctx); err != nil {
		t.Fatal(err)
	}
	ping, _ := json.Marshal(protocol.Envelope{Type: protocol.Ping})
	if err := conn.Write(ctx, websocket.MessageText, ping); err != nil {
		t.Fatal(err)
	}
	_, pong, err := conn.Read(ctx)
	if err != nil || !strings.Contains(string(pong), `"pong"`) {
		t.Fatalf("pong=%s err=%v", pong, err)
	}
	invalidStart, _ := json.Marshal(protocol.Envelope{Type: protocol.HTTPResponseStart, Payload: protocol.HTTPResponsePayload{RequestID: "req_1", ContentLength: -2}})
	if err := conn.Write(ctx, websocket.MessageText, invalidStart); err != nil {
		t.Fatal(err)
	}
	if _, _, err := conn.Read(ctx); err == nil {
		t.Fatal("invalid response start remained connected")
	}
}

func TestAgentConnectAssemblesStreamingResponse(t *testing.T) {
	sessions := NewSessionManager()
	handler := &AgentConnectHandler{sessions: sessions, heartbeatInterval: time.Hour, validateToken: func(context.Context, string) bool { return true }}
	server := httptest.NewServer(handler)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": {"Bearer tkn_test"}}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	hello, _ := json.Marshal(protocol.Envelope{Type: protocol.ClientHello, Payload: protocol.ClientHelloPayload{ProtocolVersion: protocol.Version}})
	if err := conn.Write(ctx, websocket.MessageText, hello); err != nil {
		t.Fatal(err)
	}
	if _, _, err := conn.Read(ctx); err != nil {
		t.Fatal(err)
	}
	pending := sessions.RegisterRequest("", "req_1")
	start, _ := json.Marshal(protocol.Envelope{Type: protocol.HTTPResponseStart, Payload: protocol.HTTPResponsePayload{RequestID: "req_1", StatusCode: http.StatusCreated, Headers: map[string][]string{"X-Test": {"yes"}}, ContentLength: 5}})
	if err := conn.Write(ctx, websocket.MessageText, start); err != nil {
		t.Fatal(err)
	}
	chunk, _ := (protocol.BinaryFrame{Type: protocol.ResponseBodyChunk, RequestID: "req_1", Sequence: 0, Payload: []byte("hello")}).MarshalBinary()
	if err := conn.Write(ctx, websocket.MessageBinary, chunk); err != nil {
		t.Fatal(err)
	}
	end, _ := json.Marshal(protocol.Envelope{Type: protocol.HTTPResponseEnd, Payload: protocol.HTTPResponseEndPayload{RequestID: "req_1"}})
	if err := conn.Write(ctx, websocket.MessageText, end); err != nil {
		t.Fatal(err)
	}
	select {
	case response := <-pending:
		if response.StatusCode != http.StatusCreated || response.Headers["X-Test"][0] != "yes" || string(response.Body) != "hello" {
			t.Fatalf("response=%#v", response)
		}
	case <-ctx.Done():
		t.Fatal("streamed response was not delivered")
	}
}

func TestAgentConnectCleansUpClosedSession(t *testing.T) {
	sessions := NewSessionManager()
	cleanup := make(chan struct {
		sessionID string
		agentID   string
	}, 1)
	handler := &AgentConnectHandler{
		sessions: sessions,
		cleanupSession: func(_ context.Context, sessionID, agentID string) {
			cleanup <- struct {
				sessionID string
				agentID   string
			}{sessionID, agentID}
		},
		heartbeatInterval: time.Hour,
		validateToken:     func(context.Context, string) bool { return true },
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": {"Bearer tkn_test"}}})
	if err != nil {
		t.Fatal(err)
	}
	hello, _ := json.Marshal(protocol.Envelope{Type: protocol.ClientHello, Payload: protocol.ClientHelloPayload{ProtocolVersion: protocol.Version}})
	if err := conn.Write(ctx, websocket.MessageText, hello); err != nil {
		t.Fatal(err)
	}
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		Payload protocol.ServerHelloPayload `json:"payload"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(websocket.StatusNormalClosure, "done"); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-cleanup:
		if got.sessionID != response.Payload.SessionID || got.agentID != "" {
			t.Fatalf("cleanup=%#v, session=%q", got, response.Payload.SessionID)
		}
	case <-ctx.Done():
		t.Fatal("closed session was not cleaned up")
	}
	if sessions.Count() != 0 {
		t.Fatalf("sessions.Count()=%d, want 0", sessions.Count())
	}
}

func TestAgentConnectOpensTunnelForSession(t *testing.T) {
	sessions := NewSessionManager()
	opened := make(chan struct {
		sessionID string
		address   string
	}, 1)
	handler := &AgentConnectHandler{
		sessions: sessions,
		openTunnel: func(_ context.Context, sessionID, _ string, address string) (protocol.TunnelOpenedPayload, error) {
			opened <- struct {
				sessionID string
				address   string
			}{sessionID, address}
			return protocol.TunnelOpenedPayload{TunnelID: "tun_1", Subdomain: "demo", PublicURL: "https://demo.tunnel.test"}, nil
		},
		heartbeatInterval: time.Hour,
		validateToken:     func(context.Context, string) bool { return true },
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": {"Bearer tkn_test"}}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	hello, _ := json.Marshal(protocol.Envelope{Type: protocol.ClientHello, Payload: protocol.ClientHelloPayload{ProtocolVersion: protocol.Version}})
	_ = conn.Write(ctx, websocket.MessageText, hello)
	_, _, _ = conn.Read(ctx)
	open, _ := json.Marshal(protocol.Envelope{Type: protocol.OpenTunnel, Payload: protocol.OpenTunnelPayload{RequestID: "open_1", LocalAddress: "127.0.0.1:8080"}})
	_ = conn.Write(ctx, websocket.MessageText, open)
	select {
	case got := <-opened:
		if got.address != "127.0.0.1:8080" || got.sessionID == "" {
			t.Fatalf("opened=%#v", got)
		}
	case <-ctx.Done():
		t.Fatal("gateway did not open tunnel")
	}
	_, data, err := conn.Read(ctx)
	if err != nil || !strings.Contains(string(data), `"tunnel_opened"`) {
		t.Fatalf("response=%s err=%v", data, err)
	}
}
