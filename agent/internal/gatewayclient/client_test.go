package gatewayclient

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestClientSendsHelloAndAnswersPing(t *testing.T) {
	pongReceived := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tkn_test" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		ctx := r.Context()
		_, hello, err := conn.Read(ctx)
		if err != nil || !strings.Contains(string(hello), "client_hello") {
			t.Errorf("expected client hello, got %q (%v)", hello, err)
			return
		}
		response, _ := json.Marshal(envelope{Type: "server_hello", Payload: serverHello{ProtocolVersion: protocolVersion, SessionID: "ses_test"}})
		if err := conn.Write(ctx, websocket.MessageText, response); err != nil {
			return
		}
		ping, _ := json.Marshal(envelope{Type: "ping"})
		if err := conn.Write(ctx, websocket.MessageText, ping); err != nil {
			return
		}
		_, pong, err := conn.Read(ctx)
		if err == nil && strings.Contains(string(pong), "pong") {
			pongReceived <- struct{}{}
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	connected := make(chan Connected, 1)
	go func() {
		_ = New().Connect(ctx, Config{GatewayURL: "ws" + strings.TrimPrefix(server.URL, "http"), Token: "tkn_test", Version: "test"}, func(c Connected) { connected <- c })
	}()

	select {
	case connection := <-connected:
		if connection.SessionID != "ses_test" {
			t.Fatalf("session ID = %q", connection.SessionID)
		}
	case <-ctx.Done():
		t.Fatal("client did not process server hello")
	}
	select {
	case <-pongReceived:
	case <-ctx.Done():
		t.Fatal("client did not answer ping")
	}
}

func TestClientRequiresGatewayAndToken(t *testing.T) {
	err := New().Connect(context.Background(), Config{}, nil)
	if err == nil {
		t.Fatal("Connect() error = nil, want validation error")
	}
}

func TestClientOpensTunnelAfterHandshake(t *testing.T) {
	opened := make(chan openTunnelPayload, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := websocket.Accept(w, r, nil)
		defer conn.CloseNow()
		_, _, _ = conn.Read(r.Context())
		hello, _ := json.Marshal(envelope{Type: "server_hello", Payload: serverHello{ProtocolVersion: protocolVersion, SessionID: "ses_test"}})
		_ = conn.Write(r.Context(), websocket.MessageText, hello)
		_, data, err := conn.Read(r.Context())
		if err != nil {
			return
		}
		var message struct {
			Type    string            `json:"type"`
			Payload openTunnelPayload `json:"payload"`
		}
		if json.Unmarshal(data, &message) == nil && message.Type == openTunnelType {
			opened <- message.Payload
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() {
		_ = New().Connect(ctx, Config{GatewayURL: "ws" + strings.TrimPrefix(server.URL, "http"), Token: "tkn_test", Version: "test", LocalAddress: "127.0.0.1:8080"}, nil)
	}()
	select {
	case request := <-opened:
		if request.RequestID != "open_ses_test" || request.LocalAddress != "127.0.0.1:8080" {
			t.Fatalf("open tunnel=%#v", request)
		}
	case <-ctx.Done():
		t.Fatal("client did not reopen its tunnel")
	}
}

func TestClientClosesSessionWhenContextIsCancelled(t *testing.T) {
	closed := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := websocket.Accept(w, r, nil)
		defer conn.CloseNow()
		_, _, _ = conn.Read(r.Context())
		hello, _ := json.Marshal(envelope{Type: "server_hello", Payload: serverHello{ProtocolVersion: protocolVersion, SessionID: "ses_test"}})
		_ = conn.Write(r.Context(), websocket.MessageText, hello)
		_, _, _ = conn.Read(r.Context())
		closed <- struct{}{}
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- New().Connect(ctx, Config{GatewayURL: "ws" + strings.TrimPrefix(server.URL, "http"), Token: "tkn_test", Version: "test"}, nil)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("gateway did not observe a closed agent session")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("client did not stop after cancellation")
	}
}

func TestClientDeliversHTTPRequestToHandler(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := websocket.Accept(w, r, nil)
		defer conn.CloseNow()
		_, _, _ = conn.Read(r.Context())
		hello, _ := json.Marshal(envelope{Type: "server_hello", Payload: serverHello{ProtocolVersion: protocolVersion, SessionID: "ses_test"}})
		_ = conn.Write(r.Context(), websocket.MessageText, hello)
		request, _ := json.Marshal(envelope{Type: httpRequestStartType, Payload: httpRequestPayload{RequestID: "req_1", Method: "POST", Path: "/health", ContentLength: 5}})
		_ = conn.Write(r.Context(), websocket.MessageText, request)
		chunk, _ := (binaryFrame{Type: requestBodyChunk, RequestID: "req_1", Sequence: 0, Payload: []byte("hello")}).marshalBinary()
		_ = conn.Write(r.Context(), websocket.MessageBinary, chunk)
		end, _ := json.Marshal(envelope{Type: httpRequestEndType, Payload: httpRequestEndPayload{RequestID: "req_1"}})
		_ = conn.Write(r.Context(), websocket.MessageText, end)
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	received := make(chan Request, 1)
	go func() {
		_ = New().Connect(ctx, Config{GatewayURL: "ws" + strings.TrimPrefix(server.URL, "http"), Token: "tkn_test", Version: "test", OnRequest: func(request Request) { received <- request }}, nil)
	}()
	select {
	case request := <-received:
		if request.ID != "req_1" || request.Method != "POST" || request.Path != "/health" || string(request.Body) != "hello" {
			t.Fatalf("request = %#v", request)
		}
	case <-ctx.Done():
		t.Fatal("request was not delivered")
	}
}

func TestClientExecutesRequestsConcurrentlyAndCorrelatesResponses(t *testing.T) {
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_, _ = w.Write([]byte(r.URL.Path))
	}))
	defer local.Close()
	responses := make(chan map[string]string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := websocket.Accept(w, r, nil)
		defer conn.CloseNow()
		_, _, _ = conn.Read(r.Context())
		hello, _ := json.Marshal(envelope{Type: "server_hello", Payload: serverHello{ProtocolVersion: protocolVersion, SessionID: "ses_test"}})
		_ = conn.Write(r.Context(), websocket.MessageText, hello)
		for _, id := range []string{"req_one", "req_two"} {
			request, _ := json.Marshal(envelope{Type: httpRequestStartType, Payload: httpRequestPayload{RequestID: id, Method: http.MethodGet, Path: "/" + id, ContentLength: 0}})
			_ = conn.Write(r.Context(), websocket.MessageText, request)
			end, _ := json.Marshal(envelope{Type: httpRequestEndType, Payload: httpRequestEndPayload{RequestID: id}})
			_ = conn.Write(r.Context(), websocket.MessageText, end)
		}
		got := map[string]string{}
		for range 6 {
			_, data, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			frame, frameErr := parseBinaryFrame(data)
			if frameErr == nil {
				got[frame.RequestID] += string(frame.Payload)
				continue
			}
			var message struct {
				Type    string              `json:"type"`
				Payload httpResponsePayload `json:"payload"`
			}
			_ = json.Unmarshal(data, &message)
		}
		responses <- got
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	started := time.Now()
	go func() {
		_ = New().Connect(ctx, Config{GatewayURL: "ws" + strings.TrimPrefix(server.URL, "http"), Token: "tkn_test", Version: "test", LocalAddress: local.URL, MaxConcurrentRequests: 2}, nil)
	}()
	select {
	case got := <-responses:
		if time.Since(started) >= 190*time.Millisecond || got["req_one"] != "/req_one" || got["req_two"] != "/req_two" {
			t.Fatalf("responses=%#v elapsed=%s", got, time.Since(started))
		}
	case <-ctx.Done():
		t.Fatal("did not receive concurrent responses")
	}
}

func TestClientStreamsMultiMegabyteLocalResponse(t *testing.T) {
	const bodySize = 4 << 20
	body := strings.Repeat("response-data-", (bodySize+len("response-data-")-1)/len("response-data-"))
	body = body[:bodySize]
	expectedHash := sha256.Sum256([]byte(body))
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer local.Close()
	completed := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			completed <- err
			return
		}
		defer conn.CloseNow()
		conn.SetReadLimit(websocketReadLimit)
		_, _, _ = conn.Read(r.Context())
		hello, _ := json.Marshal(envelope{Type: "server_hello", Payload: serverHello{ProtocolVersion: protocolVersion, SessionID: "ses_test"}})
		if err := conn.Write(r.Context(), websocket.MessageText, hello); err != nil {
			completed <- err
			return
		}
		start, _ := json.Marshal(envelope{Type: httpRequestStartType, Payload: httpRequestPayload{RequestID: "req_large", Method: http.MethodGet, Path: "/large", ContentLength: 0}})
		if err := conn.Write(r.Context(), websocket.MessageText, start); err != nil {
			completed <- err
			return
		}
		end, _ := json.Marshal(envelope{Type: httpRequestEndType, Payload: httpRequestEndPayload{RequestID: "req_large"}})
		if err := conn.Write(r.Context(), websocket.MessageText, end); err != nil {
			completed <- err
			return
		}
		hash := sha256.New()
		var chunks uint32
		started := false
		for {
			messageType, data, err := conn.Read(r.Context())
			if err != nil {
				completed <- err
				return
			}
			if messageType == websocket.MessageBinary {
				frame, frameErr := parseBinaryFrame(data)
				if frameErr != nil || frame.Type != responseBodyChunk || frame.RequestID != "req_large" || frame.Sequence != chunks || len(frame.Payload) > 32*1024 {
					completed <- fmt.Errorf("frame=%#v err=%v", frame, frameErr)
					return
				}
				_, _ = hash.Write(frame.Payload)
				chunks++
				continue
			}
			var message struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(data, &message); err != nil {
				completed <- err
				return
			}
			switch message.Type {
			case httpResponseStartType:
				started = true
			case httpResponseEndType:
				if !started || chunks < 2 || [32]byte(hash.Sum(nil)) != expectedHash {
					completed <- fmt.Errorf("started=%v chunks=%d hash=%x", started, chunks, hash.Sum(nil))
					return
				}
				completed <- nil
				return
			}
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func() {
		_ = New().Connect(ctx, Config{GatewayURL: "ws" + strings.TrimPrefix(server.URL, "http"), Token: "tkn_test", Version: "test", LocalAddress: local.URL}, nil)
	}()
	select {
	case err := <-completed:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("large response was not fully streamed")
	}
}

func TestRunReconnectsAfterClosedSession(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		_, _, _ = conn.Read(r.Context())
		if attempts.Add(1) == 1 {
			_ = conn.Close(websocket.StatusGoingAway, "restart")
			return
		}
		response, _ := json.Marshal(envelope{Type: "server_hello", Payload: serverHello{ProtocolVersion: protocolVersion, SessionID: "ses_reconnected"}})
		_ = conn.Write(r.Context(), websocket.MessageText, response)
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := New()
	client.reconnectInitialDelay = time.Millisecond
	client.reconnectMaxDelay = 2 * time.Millisecond
	retried := 0
	var sessionMetrics SessionMetrics
	err := client.Run(ctx, Config{GatewayURL: "ws" + strings.TrimPrefix(server.URL, "http"), Token: "tkn_test", Version: "test", OnSessionMetrics: func(metrics SessionMetrics) {
		sessionMetrics = metrics
	}}, func(connection Connected) {
		if connection.SessionID != "ses_reconnected" {
			t.Errorf("session ID = %q", connection.SessionID)
		}
		cancel()
	}, func(error, time.Duration) { retried++ })
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := attempts.Load(); got != 2 {
		t.Fatalf("connection attempts = %d, want 2", got)
	}
	if retried != 1 {
		t.Fatalf("retries = %d, want 1", retried)
	}
	if sessionMetrics.ActiveSessions != 0 || sessionMetrics.ConnectionsTotal != 1 || sessionMetrics.DisconnectionsTotal != 1 {
		t.Fatalf("session metrics=%#v", sessionMetrics)
	}
}
