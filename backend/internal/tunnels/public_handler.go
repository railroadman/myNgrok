package tunnels

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/myngrok/backend/internal/gateway"
	"github.com/myngrok/backend/internal/protocol"
)

type PublicHandler struct {
	baseDomain string
	registry   *Registry
	sessions   *gateway.SessionManager
	timeout    time.Duration
	limiter    *publicRateLimiter
}

func NewPublicHandler(baseDomain string, registry *Registry, sessions *gateway.SessionManager) *PublicHandler {
	return NewPublicHandlerWithTimeout(baseDomain, registry, sessions, 30*time.Second)
}
func NewPublicHandlerWithTimeout(baseDomain string, registry *Registry, sessions *gateway.SessionManager, timeout time.Duration) *PublicHandler {
	return &PublicHandler{baseDomain: baseDomain, registry: registry, sessions: sessions, timeout: timeout, limiter: newPublicRateLimiter(publicRateLimit, publicRateWindow)}
}
func (h *PublicHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	subdomain, ok := ParsePublicHost(r.Host, h.baseDomain)
	if !ok {
		http.NotFound(w, r)
		return
	}
	tunnel, ok := h.registry.Get(subdomain)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if allowed, retryAfter := h.limiter.allow(tunnel.ID+":"+requestClientIP(r), time.Now()); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(max(1, int(retryAfter.Seconds()))))
		http.Error(w, "public request rate limit exceeded", http.StatusTooManyRequests)
		return
	}
	if r.ContentLength > maxTunnelBodyBytes {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxTunnelBodyBytes)
	requestID, err := publicRequestID()
	if err != nil {
		http.Error(w, "request ID generation failed", 500)
		return
	}
	message, _ := json.Marshal(protocol.Envelope{Type: protocol.HTTPRequestStart, Payload: protocol.HTTPRequestPayload{
		RequestID:     requestID,
		Method:        r.Method,
		Path:          r.URL.RequestURI(),
		Headers:       publicRequestHeaders(r, requestID),
		ContentLength: r.ContentLength,
	}})
	responseCh := h.sessions.RegisterRequest(tunnel.SessionID, requestID)
	defer h.sessions.CancelRequest(requestID)
	if !h.sessions.SendContext(r.Context(), tunnel.SessionID, message) {
		http.Error(w, "tunnel agent is unavailable", http.StatusBadGateway)
		return
	}
	if err := h.sendRequestBody(r.Context(), tunnel.SessionID, requestID, r.Body); err != nil {
		h.sendCancel(tunnel.SessionID, requestID)
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	select {
	case response := <-responseCh:
		if response.Error != "" {
			http.Error(w, response.Error, http.StatusBadGateway)
			return
		}
		h.registry.RecordTraffic(tunnel.ID, r.ContentLength, int64(len(response.Body)))
		for name, values := range withoutHopByHopHeaders(response.Headers) {
			for _, value := range values {
				w.Header().Add(name, value)
			}
		}
		w.WriteHeader(response.StatusCode)
		_, _ = w.Write(response.Body)
	case <-r.Context().Done():
		h.sendCancel(tunnel.SessionID, requestID)
		http.Error(w, "tunnel response timed out", http.StatusGatewayTimeout)
	case <-time.After(h.timeout):
		h.sendCancel(tunnel.SessionID, requestID)
		http.Error(w, "tunnel response timed out", http.StatusGatewayTimeout)
	}
}

const (
	requestBodyChunkSize = 32 * 1024
	maxTunnelBodyBytes   = 32 << 20
)

func (h *PublicHandler) sendRequestBody(ctx context.Context, sessionID, requestID string, body io.Reader) error {
	buffer := make([]byte, requestBodyChunkSize)
	var sequence uint32
	for {
		count, err := body.Read(buffer)
		if count > 0 {
			frame, marshalErr := (protocol.BinaryFrame{Type: protocol.RequestBodyChunk, RequestID: requestID, Sequence: sequence, Payload: buffer[:count]}).MarshalBinary()
			if marshalErr != nil {
				return marshalErr
			}
			if !h.sessions.SendBinaryContext(ctx, sessionID, frame) {
				return fmt.Errorf("tunnel agent is unavailable")
			}
			sequence++
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	end, err := json.Marshal(protocol.Envelope{Type: protocol.HTTPRequestEnd, Payload: protocol.HTTPRequestEndPayload{RequestID: requestID}})
	if err != nil {
		return err
	}
	if !h.sessions.SendContext(ctx, sessionID, end) {
		return fmt.Errorf("tunnel agent is unavailable")
	}
	return nil
}

func (h *PublicHandler) sendCancel(sessionID, requestID string) {
	message, err := json.Marshal(protocol.Envelope{Type: protocol.CancelRequest, Payload: protocol.CancelRequestPayload{RequestID: requestID}})
	if err == nil {
		h.sessions.Send(sessionID, message)
	}
}

func publicRequestHeaders(r *http.Request, requestID string) http.Header {
	headers := withoutHopByHopHeaders(r.Header)
	for name := range headers {
		if strings.HasPrefix(strings.ToLower(name), "x-forwarded-") || strings.EqualFold(name, "Forwarded") {
			delete(headers, name)
		}
	}
	clientIP := requestClientIP(r)
	if clientIP != "" {
		headers.Set("X-Forwarded-For", clientIP)
	}
	headers.Set("X-Forwarded-Host", r.Host)
	if r.TLS == nil {
		headers.Set("X-Forwarded-Proto", "http")
	} else {
		headers.Set("X-Forwarded-Proto", "https")
	}
	headers.Set("X-Tunnel-Request-ID", requestID)
	return headers
}

func requestClientIP(r *http.Request) string {
	clientIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		clientIP = r.RemoteAddr
	}
	if clientIP == "" {
		return "unknown"
	}
	return clientIP
}

func withoutHopByHopHeaders(headers http.Header) http.Header {
	filtered := headers.Clone()
	for _, value := range filtered.Values("Connection") {
		for _, name := range strings.Split(value, ",") {
			filtered.Del(strings.TrimSpace(name))
		}
	}
	for _, name := range []string{"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "TE", "Trailer", "Transfer-Encoding", "Upgrade"} {
		filtered.Del(name)
	}
	return filtered
}
func publicRequestID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "req_" + hex.EncodeToString(b), nil
}
