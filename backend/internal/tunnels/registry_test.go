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

func TestRegistryTracksPerUserTrafficDeltasAndDrains(t *testing.T) {
	registry := NewRegistry()
	registry.Open(ActiveTunnel{ID: "tun_1", Subdomain: "one", UserID: "user_a"})
	registry.Open(ActiveTunnel{ID: "tun_2", Subdomain: "two", UserID: "user_b"})
	registry.RecordTraffic("tun_1", 12, 34)
	registry.RecordTraffic("tun_1", 8, 16)
	registry.RecordTraffic("tun_2", 100, 200)

	deltas := registry.DrainUserDeltas()
	if got := deltas["user_a"]; got.RequestsTotal != 2 || got.RequestBytes != 20 || got.ResponseBytes != 50 {
		t.Fatalf("user_a delta=%#v", got)
	}
	if got := deltas["user_b"]; got.RequestsTotal != 1 || got.RequestBytes != 100 || got.ResponseBytes != 200 {
		t.Fatalf("user_b delta=%#v", got)
	}

	// A second drain immediately after must be empty: deltas are consumed, not accumulated forever.
	if again := registry.DrainUserDeltas(); len(again) != 0 {
		t.Fatalf("expected drained deltas to reset, got %#v", again)
	}

	registry.RecordTraffic("tun_1", 5, 5)
	if got := registry.DrainUserDeltas()["user_a"]; got.RequestsTotal != 1 || got.RequestBytes != 5 || got.ResponseBytes != 5 {
		t.Fatalf("post-drain delta=%#v", got)
	}
}
