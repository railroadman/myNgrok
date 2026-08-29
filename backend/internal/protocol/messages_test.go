package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestServerHelloUsesProtocolV1WireFormat(t *testing.T) {
	message := Envelope{Type: ServerHello, Payload: ServerHelloPayload{
		ProtocolVersion:          Version,
		SessionID:                "ses_test",
		HeartbeatIntervalSeconds: 20,
	}}
	encoded, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if decoded["type"] != ServerHello {
		t.Fatalf("type = %v, want %q", decoded["type"], ServerHello)
	}
}

func TestOpenTunnelUsesExpectedWireFormat(t *testing.T) {
	encoded, err := json.Marshal(Envelope{Type: OpenTunnel, Payload: OpenTunnelPayload{RequestID: "req_1", LocalAddress: "127.0.0.1:8080"}})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	payload := decoded["payload"].(map[string]any)
	if decoded["type"] != OpenTunnel || payload["localAddress"] != "127.0.0.1:8080" {
		t.Fatalf("unexpected protocol message: %s", encoded)
	}
}

func TestTunnelOpenedUsesExpectedWireFormat(t *testing.T) {
	encoded, err := json.Marshal(Envelope{Type: TunnelOpened, Payload: TunnelOpenedPayload{RequestID: "req_1", TunnelID: "tun_1", Subdomain: "demo123", PublicURL: "https://demo123.tunnel.example"}})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if !strings.Contains(string(encoded), `"publicUrl":"https://demo123.tunnel.example"`) {
		t.Fatalf("unexpected response: %s", encoded)
	}
}

func TestCloseTunnelUsesExpectedWireFormat(t *testing.T) {
	encoded, err := json.Marshal(Envelope{Type: CloseTunnel, Payload: CloseTunnelPayload{RequestID: "req_2", TunnelID: "tun_1"}})
	if err != nil || !strings.Contains(string(encoded), `"tunnelId":"tun_1"`) {
		t.Fatalf("message = %s, error = %v", encoded, err)
	}
}
