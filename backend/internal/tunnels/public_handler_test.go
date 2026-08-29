package tunnels

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"github.com/myngrok/backend/internal/gateway"
	"github.com/myngrok/backend/internal/protocol"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestPublicHandlerReturnsDeliveredAgentResponse(t *testing.T) {
	registry := NewRegistry()
	sessions := gateway.NewSessionManager()
	outbound := make(chan gateway.OutboundMessage, 2)
	sessions.Register(gateway.Session{ID: "ses_1", Outbound: outbound})
	registry.Open(ActiveTunnel{ID: "tun_1", Subdomain: "demo123", SessionID: "ses_1"})
	go func() {
		message := (<-outbound).Data
		var envelope struct {
			Payload struct {
				RequestID string `json:"requestId"`
			} `json:"payload"`
		}
		_ = json.Unmarshal(message, &envelope)
		sessions.DeliverResponse(envelope.Payload.RequestID, gateway.Response{StatusCode: 201, Body: []byte("from agent")})
	}()
	handler := NewPublicHandler("tunnel.example.test", registry, sessions)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "http://demo123.tunnel.example.test/hello", nil)
	request.Host = "demo123.tunnel.example.test"
	handler.ServeHTTP(response, request)
	if response.Code != 201 || response.Body.String() != "from agent" {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestPublicHandlerReturnsBadGatewayWhenAgentDisconnects(t *testing.T) {
	registry := NewRegistry()
	sessions := gateway.NewSessionManager()
	outbound := make(chan gateway.OutboundMessage, 2)
	sessions.Register(gateway.Session{ID: "ses_1", Outbound: outbound})
	registry.Open(ActiveTunnel{ID: "tun_1", Subdomain: "demo123", SessionID: "ses_1"})
	go func() {
		<-outbound
		<-outbound
		sessions.Remove("ses_1")
	}()
	handler := NewPublicHandlerWithTimeout("tunnel.example.test", registry, sessions, time.Second)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "http://demo123.tunnel.example.test/hello", nil)
	request.Host = "demo123.tunnel.example.test"
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadGateway || !strings.Contains(response.Body.String(), "tunnel agent disconnected") {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestPublicHandlerForwardsRequestBody(t *testing.T) {
	registry := NewRegistry()
	sessions := gateway.NewSessionManager()
	outbound := make(chan gateway.OutboundMessage, 2)
	sessions.Register(gateway.Session{ID: "ses_1", Outbound: outbound})
	registry.Open(ActiveTunnel{ID: "tun_1", Subdomain: "demo123", SessionID: "ses_1"})
	go func() {
		message := (<-outbound).Data
		var envelope struct {
			Payload protocol.HTTPRequestPayload `json:"payload"`
		}
		if err := json.Unmarshal(message, &envelope); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if envelope.Payload.ContentLength != int64(len("request payload")) {
			t.Errorf("content length=%d", envelope.Payload.ContentLength)
		}
		chunk := <-outbound
		if chunk.Type != websocket.MessageBinary {
			t.Errorf("chunk type=%v", chunk.Type)
		}
		frame, err := protocol.ParseBinaryFrame(chunk.Data)
		if err != nil || frame.Type != protocol.RequestBodyChunk || string(frame.Payload) != "request payload" || frame.Sequence != 0 {
			t.Errorf("frame=%#v err=%v", frame, err)
		}
		end := <-outbound
		var endEnvelope struct {
			Type    string                         `json:"type"`
			Payload protocol.HTTPRequestEndPayload `json:"payload"`
		}
		if err := json.Unmarshal(end.Data, &endEnvelope); err != nil || endEnvelope.Type != protocol.HTTPRequestEnd || endEnvelope.Payload.RequestID != envelope.Payload.RequestID {
			t.Errorf("end=%s err=%v", end.Data, err)
		}
		sessions.DeliverResponse(envelope.Payload.RequestID, gateway.Response{StatusCode: http.StatusNoContent})
	}()
	handler := NewPublicHandler("tunnel.example.test", registry, sessions)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "http://demo123.tunnel.example.test/", strings.NewReader("request payload"))
	request.Host = "demo123.tunnel.example.test"
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d", response.Code)
	}
}

func TestPublicHandlerRejectsOversizedRequestBody(t *testing.T) {
	registry := NewRegistry()
	registry.Open(ActiveTunnel{ID: "tun_1", Subdomain: "demo123", SessionID: "ses_1"})
	handler := NewPublicHandler("tunnel.example.test", registry, gateway.NewSessionManager())
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "http://demo123.tunnel.example.test/upload", strings.NewReader("x"))
	request.Host = "demo123.tunnel.example.test"
	request.ContentLength = maxTunnelBodyBytes + 1
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d", response.Code)
	}
}

func TestPublicHandlerRejectsUnknownOrUnavailableTunnel(t *testing.T) {
	handler := NewPublicHandler("tunnel.example.test", NewRegistry(), gateway.NewSessionManager())
	unknown := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "http://missing.tunnel.example.test/", nil)
	request.Host = "missing.tunnel.example.test"
	handler.ServeHTTP(unknown, request)
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown tunnel status=%d", unknown.Code)
	}

	registry := NewRegistry()
	registry.Open(ActiveTunnel{ID: "tun_1", Subdomain: "demo123", SessionID: "missing-session"})
	handler = NewPublicHandler("tunnel.example.test", registry, gateway.NewSessionManager())
	unavailable := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "http://demo123.tunnel.example.test/", nil)
	request.Host = "demo123.tunnel.example.test"
	handler.ServeHTTP(unavailable, request)
	if unavailable.Code != http.StatusBadGateway || !strings.Contains(unavailable.Body.String(), "unavailable") {
		t.Fatalf("unavailable status=%d body=%s", unavailable.Code, unavailable.Body.String())
	}
}

func TestPublicHandlerTimeoutSendsCancellation(t *testing.T) {
	registry := NewRegistry()
	sessions := gateway.NewSessionManager()
	outbound := make(chan gateway.OutboundMessage, 3)
	sessions.Register(gateway.Session{ID: "ses_1", Outbound: outbound})
	registry.Open(ActiveTunnel{ID: "tun_1", Subdomain: "demo123", SessionID: "ses_1"})
	handler := NewPublicHandlerWithTimeout("tunnel.example.test", registry, sessions, 10*time.Millisecond)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "http://demo123.tunnel.example.test/", nil)
	request.Host = "demo123.tunnel.example.test"
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusGatewayTimeout {
		t.Fatalf("timeout status=%d", response.Code)
	}
	<-outbound // request start
	<-outbound // request end
	cancel := <-outbound
	if !strings.Contains(string(cancel.Data), `"cancel_request"`) {
		t.Fatalf("cancel message=%s", cancel.Data)
	}
}

func TestPublicHandlerRateLimitsOneTunnelAndClient(t *testing.T) {
	registry := NewRegistry()
	registry.Open(ActiveTunnel{ID: "tun_limited", Subdomain: "limited", SessionID: "missing-session"})
	handler := NewPublicHandler("tunnel.example.test", registry, gateway.NewSessionManager())
	for requestNumber := 0; requestNumber < publicRateLimit; requestNumber++ {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "http://limited.tunnel.example.test/", nil)
		request.Host = "limited.tunnel.example.test"
		request.RemoteAddr = "203.0.113.1:1234"
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadGateway {
			t.Fatalf("request %d status=%d", requestNumber, response.Code)
		}
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "http://limited.tunnel.example.test/", nil)
	request.Host = "limited.tunnel.example.test"
	request.RemoteAddr = "203.0.113.1:5678"
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") == "" {
		t.Fatalf("rate-limited status=%d headers=%#v", response.Code, response.Header())
	}
}

func TestSendRequestBodySplitsBinaryFrames(t *testing.T) {
	sessions := gateway.NewSessionManager()
	outbound := make(chan gateway.OutboundMessage, 4)
	sessions.Register(gateway.Session{ID: "ses_1", Outbound: outbound})
	handler := NewPublicHandler("tunnel.example.test", NewRegistry(), sessions)
	body := strings.Repeat("a", requestBodyChunkSize+5)
	if err := handler.sendRequestBody(context.Background(), "ses_1", "req_1", strings.NewReader(body)); err != nil {
		t.Fatal(err)
	}
	for sequence, expectedSize := range []int{requestBodyChunkSize, 5} {
		message := <-outbound
		frame, err := protocol.ParseBinaryFrame(message.Data)
		if message.Type != websocket.MessageBinary || err != nil || frame.Sequence != uint32(sequence) || len(frame.Payload) != expectedSize {
			t.Fatalf("message=%#v frame=%#v err=%v", message, frame, err)
		}
	}
	end := <-outbound
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(end.Data, &envelope); err != nil || envelope.Type != protocol.HTTPRequestEnd {
		t.Fatalf("end=%s err=%v", end.Data, err)
	}
}

func TestPublicHandlerStreamsMultiMegabyteRequestBody(t *testing.T) {
	const bodySize = 4 << 20
	body := strings.Repeat("stream-data-", (bodySize+len("stream-data-")-1)/len("stream-data-"))
	body = body[:bodySize]
	expectedHash := sha256.Sum256([]byte(body))
	registry := NewRegistry()
	sessions := gateway.NewSessionManager()
	outbound := make(chan gateway.OutboundMessage, 2)
	sessions.Register(gateway.Session{ID: "ses_1", Outbound: outbound})
	registry.Open(ActiveTunnel{ID: "tun_1", Subdomain: "demo123", SessionID: "ses_1"})
	done := make(chan struct{})
	go func() {
		defer close(done)
		start := <-outbound
		var envelope struct {
			Payload protocol.HTTPRequestPayload `json:"payload"`
		}
		if err := json.Unmarshal(start.Data, &envelope); err != nil || envelope.Payload.ContentLength != bodySize {
			t.Errorf("start=%s err=%v", start.Data, err)
			return
		}
		hash := sha256.New()
		var chunks int
		for {
			message := <-outbound
			if message.Type == websocket.MessageBinary {
				frame, err := protocol.ParseBinaryFrame(message.Data)
				if err != nil || frame.Sequence != uint32(chunks) || len(frame.Payload) > requestBodyChunkSize {
					t.Errorf("frame=%#v err=%v", frame, err)
					return
				}
				_, _ = hash.Write(frame.Payload)
				chunks++
				continue
			}
			var end struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(message.Data, &end); err != nil || end.Type != protocol.HTTPRequestEnd {
				t.Errorf("end=%s err=%v", message.Data, err)
				return
			}
			if chunks < 2 || [32]byte(hash.Sum(nil)) != expectedHash {
				t.Errorf("chunks=%d hash=%x", chunks, hash.Sum(nil))
				return
			}
			sessions.DeliverResponse(envelope.Payload.RequestID, gateway.Response{StatusCode: http.StatusNoContent})
			return
		}
	}()
	handler := NewPublicHandler("tunnel.example.test", registry, sessions)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "http://demo123.tunnel.example.test/upload", strings.NewReader(body))
	request.Host = "demo123.tunnel.example.test"
	handler.ServeHTTP(response, request)
	<-done
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d", response.Code)
	}
}

func TestPublicHandlerForwardsRequestAndResponseHeaders(t *testing.T) {
	registry := NewRegistry()
	sessions := gateway.NewSessionManager()
	outbound := make(chan gateway.OutboundMessage, 2)
	sessions.Register(gateway.Session{ID: "ses_1", Outbound: outbound})
	registry.Open(ActiveTunnel{ID: "tun_1", Subdomain: "demo123", SessionID: "ses_1"})
	go func() {
		message := (<-outbound).Data
		var envelope struct {
			Payload protocol.HTTPRequestPayload `json:"payload"`
		}
		if err := json.Unmarshal(message, &envelope); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if got := http.Header(envelope.Payload.Headers).Values("X-Trace"); len(got) != 2 || got[0] != "first" || got[1] != "second" {
			t.Errorf("forwarded headers = %#v", envelope.Payload.Headers)
		}
		sessions.DeliverResponse(envelope.Payload.RequestID, gateway.Response{StatusCode: http.StatusCreated, Headers: map[string][]string{"X-Local": {"one", "two"}}})
	}()
	handler := NewPublicHandler("tunnel.example.test", registry, sessions)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "http://demo123.tunnel.example.test/hello", nil)
	request.Host = "demo123.tunnel.example.test"
	request.Header.Add("X-Trace", "first")
	request.Header.Add("X-Trace", "second")
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || response.Header().Values("X-Local")[0] != "one" || response.Header().Values("X-Local")[1] != "two" {
		t.Fatalf("status=%d headers=%#v", response.Code, response.Header())
	}
}

func TestPublicRequestHeadersFiltersHopByHopAndSetsForwardedHeaders(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://demo123.tunnel.example.test/", nil)
	request.Host = "demo123.tunnel.example.test"
	request.RemoteAddr = "203.0.113.10:4567"
	request.Header.Set("Connection", "X-Remove")
	request.Header.Set("X-Remove", "secret")
	request.Header.Set("Upgrade", "websocket")
	request.Header.Set("X-Forwarded-For", "spoofed")
	headers := publicRequestHeaders(request, "req_1")
	if headers.Get("Connection") != "" || headers.Get("X-Remove") != "" || headers.Get("Upgrade") != "" {
		t.Fatalf("hop-by-hop headers were retained: %#v", headers)
	}
	if headers.Get("X-Forwarded-For") != "203.0.113.10" || headers.Get("X-Forwarded-Host") != request.Host || headers.Get("X-Forwarded-Proto") != "http" || headers.Get("X-Tunnel-Request-ID") != "req_1" {
		t.Fatalf("forwarded headers = %#v", headers)
	}
}

func TestWithoutHopByHopHeadersFiltersConnectionNamedResponseHeader(t *testing.T) {
	headers := withoutHopByHopHeaders(http.Header{"Connection": {"X-Remove"}, "X-Remove": {"secret"}, "X-Keep": {"ok"}})
	if headers.Get("Connection") != "" || headers.Get("X-Remove") != "" || headers.Get("X-Keep") != "ok" {
		t.Fatalf("headers=%#v", headers)
	}
}
