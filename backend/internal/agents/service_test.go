package agents

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAgentModelHasConnectionFields(t *testing.T) {
	agent := Agent{ID: "agent-1", InstanceID: "host-1", Connected: true}
	if !agent.Connected || agent.InstanceID == "" {
		t.Fatal("agent connection model is incomplete")
	}
}

func TestAgentUsesAPIFieldNames(t *testing.T) {
	encoded, err := json.Marshal(Agent{ID: "agent-1", InstanceID: "host-1"})
	if err != nil || !strings.Contains(string(encoded), `"instanceID"`) {
		t.Fatalf("agent JSON = %s, error = %v", encoded, err)
	}
}
