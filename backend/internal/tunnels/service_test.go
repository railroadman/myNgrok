package tunnels

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTunnelUsesAPIFieldNames(t *testing.T) {
	encoded, err := json.Marshal(Tunnel{ID: "tun_1", LocalAddress: "127.0.0.1:8080", AgentID: "agent_1"})
	if err != nil || !strings.Contains(string(encoded), `"localAddress"`) {
		t.Fatalf("JSON = %s, error = %v", encoded, err)
	}
}
