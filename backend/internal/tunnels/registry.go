package tunnels

import "sync"

// ActiveTunnel is the in-memory routing record used only while a tunnel is open.
type ActiveTunnel struct {
	ID, Subdomain, AgentID, UserID, SessionID, LocalAddress string
}

type Registry struct {
	mu          sync.RWMutex
	bySubdomain map[string]ActiveTunnel
	byID        map[string]string
	traffic     TrafficMetrics
	perUser     map[string]TrafficMetrics
}

type TrafficMetrics struct {
	RequestsTotal uint64
	RequestBytes  uint64
	ResponseBytes uint64
}

func NewRegistry() *Registry {
	return &Registry{bySubdomain: make(map[string]ActiveTunnel), byID: make(map[string]string), perUser: make(map[string]TrafficMetrics)}
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
	subdomain, exists := r.byID[tunnelID]
	if !exists {
		return
	}
	r.traffic.RequestsTotal++
	if requestBytes > 0 {
		r.traffic.RequestBytes += uint64(requestBytes)
	}
	if responseBytes > 0 {
		r.traffic.ResponseBytes += uint64(responseBytes)
	}
	if userID := r.bySubdomain[subdomain].UserID; userID != "" {
		delta := r.perUser[userID]
		delta.RequestsTotal++
		if requestBytes > 0 {
			delta.RequestBytes += uint64(requestBytes)
		}
		if responseBytes > 0 {
			delta.ResponseBytes += uint64(responseBytes)
		}
		r.perUser[userID] = delta
	}
}

func (r *Registry) TrafficMetrics() TrafficMetrics {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.traffic
}

// DrainUserDeltas returns the accumulated per-user traffic since the last
// drain and resets the in-memory deltas, so callers can flush them to
// persistent storage without double-counting.
func (r *Registry) DrainUserDeltas() map[string]TrafficMetrics {
	r.mu.Lock()
	defer r.mu.Unlock()
	drained := r.perUser
	r.perUser = make(map[string]TrafficMetrics)
	return drained
}
