package tunnels

import "testing"

func TestRegistryRoutesAndClosesTunnel(t *testing.T) {
	registry := NewRegistry()
	registry.Open(ActiveTunnel{ID: "tun_1", Subdomain: "demo123", AgentID: "agent_1", SessionID: "ses_1", LocalAddress: "127.0.0.1:8080"})
	tunnel, ok := registry.Get("demo123")
	if !ok || tunnel.ID != "tun_1" || registry.Count() != 1 {
		t.Fatalf("tunnel was not registered: %#v", tunnel)
	}
	if !registry.Close("tun_1") || registry.Count() != 0 {
		t.Fatal("tunnel was not removed")
	}
	if registry.Close("missing") {
		t.Fatal("unknown tunnel was reported as closed")
	}
}

func TestRegistryClosesAllTunnelsForDisconnectedSession(t *testing.T) {
	registry := NewRegistry()
	registry.Open(ActiveTunnel{ID: "tun_1", Subdomain: "one", SessionID: "ses_1"})
	registry.Open(ActiveTunnel{ID: "tun_2", Subdomain: "two", SessionID: "ses_1"})
	registry.Open(ActiveTunnel{ID: "tun_3", Subdomain: "three", SessionID: "ses_2"})
	registry.CloseSession("ses_1")
	if registry.Count() != 1 {
		t.Fatalf("Count() = %d, want 1", registry.Count())
	}
	if _, ok := registry.Get("three"); !ok {
		t.Fatal("other session tunnel was removed")
	}
}

func TestRegistryTracksTrafficForActiveTunnel(t *testing.T) {
	registry := NewRegistry()
	registry.Open(ActiveTunnel{ID: "tun_1", Subdomain: "one"})
	registry.RecordTraffic("tun_1", 12, 34)
	registry.RecordTraffic("missing", 99, 99)
	if got := registry.TrafficMetrics(); got.RequestsTotal != 1 || got.RequestBytes != 12 || got.ResponseBytes != 34 {
		t.Fatalf("traffic=%#v", got)
	}
}
