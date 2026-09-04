package traffic

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/myngrok/backend/internal/auth"
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
	if r.URL.Path != "/api/v1/traffic" || r.Method != http.MethodGet {
		writeError(w, 404, "NOT_FOUND", "Route not found")
		return
	}
	totals, err := h.service.GetTotals(r.Context(), user.ID)
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Unable to load traffic totals")
		return
	}
	writeData(w, 200, totals)
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
