package tunnels

import "sync"

// ActiveTunnel is the in-memory routing record used only while a tunnel is open.
type ActiveTunnel struct {
	ID, Subdomain, AgentID, SessionID, LocalAddress string
}

type Registry struct {
	mu          sync.RWMutex
	bySubdomain map[string]ActiveTunnel
	byID        map[string]string
	traffic     TrafficMetrics
}

type TrafficMetrics struct {
	RequestsTotal uint64
	RequestBytes  uint64
	ResponseBytes uint64
}

func NewRegistry() *Registry {
	return &Registry{bySubdomain: make(map[string]ActiveTunnel), byID: make(map[string]string)}
}

func (r *Registry) Open(tunnel ActiveTunnel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bySubdomain[tunnel.Subdomain] = tunnel
	r.byID[tunnel.ID] = tunnel.Subdomain
}
func (r *Registry) Get(subdomain string) (ActiveTunnel, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tunnel, ok := r.bySubdomain[subdomain]
	return tunnel, ok
}
func (r *Registry) Close(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	subdomain, ok := r.byID[id]
	if !ok {
		return false
	}
	delete(r.byID, id)
	delete(r.bySubdomain, subdomain)
	return true
}
func (r *Registry) CloseSession(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, subdomain := range r.byID {
		if r.bySubdomain[subdomain].SessionID == sessionID {
			delete(r.byID, id)
			delete(r.bySubdomain, subdomain)
		}
	}
}
func (r *Registry) Count() int { r.mu.RLock(); defer r.mu.RUnlock(); return len(r.byID) }

func (r *Registry) RecordTraffic(tunnelID string, requestBytes, responseBytes int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byID[tunnelID]; !exists {
		return
	}
	r.traffic.RequestsTotal++
	if requestBytes > 0 {
		r.traffic.RequestBytes += uint64(requestBytes)
	}
	if responseBytes > 0 {
		r.traffic.ResponseBytes += uint64(responseBytes)
	}
}

func (r *Registry) TrafficMetrics() TrafficMetrics {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.traffic
}
