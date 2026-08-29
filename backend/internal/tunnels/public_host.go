package tunnels

import (
	"net"
	"strings"
)

// ParsePublicHost extracts a one-label tunnel subdomain from an HTTP Host
// header. It deliberately rejects the bare base domain and nested labels.
func ParsePublicHost(host, baseDomain string) (string, bool) {
	host = normalizeHost(host)
	baseDomain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(baseDomain), "."))
	if host == "" || baseDomain == "" {
		return "", false
	}
	suffix := "." + baseDomain
	if !strings.HasSuffix(host, suffix) {
		return "", false
	}
	subdomain := strings.TrimSuffix(host, suffix)
	if subdomain == "" || strings.Contains(subdomain, ".") {
		return "", false
	}
	for _, char := range subdomain {
		if !(char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '-') {
			return "", false
		}
	}
	return subdomain, true
}

func normalizeHost(host string) string {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if value, _, err := net.SplitHostPort(host); err == nil {
		return value
	}
	return host
}
