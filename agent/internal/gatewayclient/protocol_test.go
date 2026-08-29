package gatewayclient

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOpenTunnelMessageHasSharedWireFields(t *testing.T) {
	encoded, err := json.Marshal(envelope{Type: openTunnelType, Payload: openTunnelPayload{RequestID: "req_1", LocalAddress: "127.0.0.1:8080"}})
	if err != nil || !strings.Contains(string(encoded), `"localAddress":"127.0.0.1:8080"`) {
		t.Fatalf("message = %s, error = %v", encoded, err)
	}
}

func TestTunnelOpenedMessageHasSharedWireFields(t *testing.T) {
	encoded, err := json.Marshal(envelope{Type: tunnelOpenedType, Payload: tunnelOpenedPayload{RequestID: "req_1", TunnelID: "tun_1", PublicURL: "https://demo123.tunnel.example"}})
	if err != nil || !strings.Contains(string(encoded), `"tunnelId":"tun_1"`) {
		t.Fatalf("message = %s, error = %v", encoded, err)
	}
}

func TestCloseTunnelMessageHasSharedWireFields(t *testing.T) {
	encoded, err := json.Marshal(envelope{Type: closeTunnelType, Payload: closeTunnelPayload{RequestID: "req_2", TunnelID: "tun_1"}})
	if err != nil || !strings.Contains(string(encoded), `"tunnelId":"tun_1"`) {
		t.Fatalf("message = %s, error = %v", encoded, err)
	}
}
