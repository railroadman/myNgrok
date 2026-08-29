package agents

import (
	"github.com/myngrok/backend/internal/auth"
	"net/http"
	"strings"
)

type HTTPHandler struct {
	service *Service
	auth    *auth.Service
}

func NewHTTPHandler(service *Service, authenticator *auth.Service) *HTTPHandler {
	return &HTTPHandler{service, authenticator}
}
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user, err := h.auth.UserFromAccessToken(r.Context(), strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")))
	if err != nil {
		writeError(w, 401, "UNAUTHORIZED", "Authentication required")
		return
	}
	if r.URL.Path != "/api/v1/agents" || r.Method != http.MethodGet {
		writeError(w, 404, "NOT_FOUND", "Route not found")
		return
	}
	items, err := h.service.List(r.Context(), user.ID)
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Unable to load agents")
		return
	}
	writeData(w, 200, items)
}
