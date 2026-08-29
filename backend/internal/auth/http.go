package auth

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type HTTPHandler struct {
	service       *Service
	secureCookies bool
	limiter       *authRateLimiter
}

const (
	refreshCookieName       = "refresh_token"
	secureRefreshCookieName = "__Host-refresh_token"
)

func NewHTTPHandler(service *Service, secureCookies bool) *HTTPHandler {
	return &HTTPHandler{service: service, secureCookies: secureCookies, limiter: newAuthRateLimiter(authRateLimit, authRateWindow)}
}
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.isRateLimitedRoute(r) {
		allowed, retryAfter := h.limiter.allow(remoteIP(r), time.Now())
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(max(1, int(retryAfter.Seconds()))))
			writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many authentication attempts")
			return
		}
	}
	switch r.Method + " " + r.URL.Path {
	case "POST /api/v1/auth/register":
		h.register(w, r)
	case "POST /api/v1/auth/login":
		h.login(w, r)
	case "POST /api/v1/auth/refresh":
		h.refresh(w, r)
	case "POST /api/v1/auth/logout":
		h.logout(w, r)
	case "GET /api/v1/auth/me":
		h.me(w, r)
	default:
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Route not found")
	}
}

func (h *HTTPHandler) isRateLimitedRoute(r *http.Request) bool {
	return r.Method == http.MethodPost && (r.URL.Path == "/api/v1/auth/register" || r.URL.Path == "/api/v1/auth/login" || r.URL.Path == "/api/v1/auth/refresh")
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *HTTPHandler) register(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !decode(w, r, &in) {
		return
	}
	user, err := h.service.Register(r.Context(), in.Email, in.Password)
	if err != nil {
		if errors.Is(err, ErrEmailTaken) {
			writeError(w, 409, "EMAIL_TAKEN", "Email is already registered")
		} else {
			writeError(w, 400, "INVALID_REGISTRATION", "Email or password is invalid")
		}
		return
	}
	writeData(w, http.StatusCreated, map[string]any{"id": user.ID, "email": user.Email})
}
func (h *HTTPHandler) login(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !decode(w, r, &in) {
		return
	}
	user, tokens, err := h.service.Login(r.Context(), in.Email, in.Password, r.UserAgent(), remoteIP(r))
	if err != nil {
		writeError(w, 401, "INVALID_CREDENTIALS", "Invalid email or password")
		return
	}
	h.setRefreshCookie(w, tokens)
	writeData(w, http.StatusOK, map[string]any{"accessToken": tokens.AccessToken, "user": map[string]string{"id": user.ID, "email": user.Email}})
}
func (h *HTTPHandler) refresh(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(h.refreshCookieName())
	raw := ""
	if cookie != nil {
		raw = cookie.Value
	}
	user, tokens, err := h.service.Refresh(r.Context(), raw, r.UserAgent(), remoteIP(r))
	if err != nil {
		h.clearRefreshCookie(w)
		writeError(w, 401, "UNAUTHORIZED", "Authentication required")
		return
	}
	h.setRefreshCookie(w, tokens)
	writeData(w, http.StatusOK, map[string]any{"accessToken": tokens.AccessToken, "user": map[string]string{"id": user.ID, "email": user.Email}})
}
func (h *HTTPHandler) logout(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie("refresh_token")
	if cookie != nil {
		_ = h.service.Logout(r.Context(), cookie.Value)
	}
	h.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}
func (h *HTTPHandler) me(w http.ResponseWriter, r *http.Request) {
	user, err := h.service.UserFromAccessToken(r.Context(), bearer(r))
	if err != nil {
		writeError(w, 401, "UNAUTHORIZED", "Authentication required")
		return
	}
	writeData(w, http.StatusOK, map[string]string{"id": user.ID, "email": user.Email})
}
func (h *HTTPHandler) setRefreshCookie(w http.ResponseWriter, t Tokens) {
	http.SetCookie(w, &http.Cookie{Name: h.refreshCookieName(), Value: t.RefreshToken, Path: "/", HttpOnly: true, Secure: h.secureCookies, SameSite: http.SameSiteStrictMode, Expires: t.RefreshExpiresAt})
}
func (h *HTTPHandler) clearRefreshCookie(w http.ResponseWriter) {
	for _, name := range []string{refreshCookieName, secureRefreshCookieName} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", HttpOnly: true, Secure: h.secureCookies, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	}
}

func (h *HTTPHandler) refreshCookieName() string {
	if h.secureCookies {
		return secureRefreshCookieName
	}
	return refreshCookieName
}
func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		writeError(w, 400, "INVALID_JSON", "Request body must be valid JSON")
		return false
	}
	return true
}
func bearer(r *http.Request) string {
	return strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
}
func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return ""
	}
	return host
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
