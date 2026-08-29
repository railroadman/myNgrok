package agenttokens

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/myngrok/backend/internal/auth"
	"github.com/myngrok/backend/internal/gateway"
)

type HTTPHandler struct {
	service  *Service
	auth     *auth.Service
	sessions *gateway.SessionManager
}

func NewHTTPHandler(service *Service, authenticator *auth.Service, sessions *gateway.SessionManager) *HTTPHandler {
	return &HTTPHandler{service: service, auth: authenticator, sessions: sessions}
}
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user, err := h.auth.UserFromAccessToken(r.Context(), bearer(r))
	if err != nil {
		writeError(w, 401, "UNAUTHORIZED", "Authentication required")
		return
	}
	if r.URL.Path == "/api/v1/agent-tokens" && r.Method == http.MethodGet {
		h.list(w, r, user)
		return
	}
	if r.URL.Path == "/api/v1/agent-tokens" && r.Method == http.MethodPost {
		h.create(w, r, user)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/v1/agent-tokens/") && r.Method == http.MethodDelete {
		h.revoke(w, r, user)
		return
	}
	writeError(w, 404, "NOT_FOUND", "Route not found")
}
func (h *HTTPHandler) list(w http.ResponseWriter, r *http.Request, user auth.User) {
	tokens, err := h.service.List(r.Context(), user.ID)
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Unable to load tokens")
		return
	}
	writeData(w, 200, tokens)
}
func (h *HTTPHandler) create(w http.ResponseWriter, r *http.Request, user auth.User) {
	var in struct {
		Name string `json:"name"`
	}
	if r.Body = http.MaxBytesReader(w, r.Body, 16<<10); json.NewDecoder(r.Body).Decode(&in) != nil {
		writeError(w, 400, "INVALID_JSON", "Request body must be valid JSON")
		return
	}
	token, err := h.service.Create(r.Context(), user.ID, in.Name)
	if err != nil {
		writeError(w, 400, "INVALID_TOKEN_NAME", err.Error())
		return
	}
	writeData(w, 201, map[string]any{"id": token.ID, "name": token.Name, "prefix": token.Prefix, "token": token.Plaintext, "createdAt": token.CreatedAt})
}
func (h *HTTPHandler) revoke(w http.ResponseWriter, r *http.Request, user auth.User) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/agent-tokens/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, 404, "NOT_FOUND", "Route not found")
		return
	}
	tokenHash, ok, err := h.service.Revoke(r.Context(), user.ID, id)
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Unable to revoke token")
		return
	}
	if !ok {
		writeError(w, 404, "NOT_FOUND", "Token not found")
		return
	}
	if h.sessions != nil {
		h.sessions.DisconnectTokenHash(tokenHash)
	}
	w.WriteHeader(http.StatusNoContent)
}
func bearer(r *http.Request) string {
	return strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
}
func writeData(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": message}})
}
