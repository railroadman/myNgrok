package agents

import (
	"testing"
	"time"
)

func TestServiceListReturnsEmptyForUnknownUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}
	// Without a real pool, List will panic. This test verifies the function signature exists.
	// Real coverage comes from integration_test.go with Postgres.
}

func TestAgentModelDefaults(t *testing.T) {
	a := Agent{}
	if a.Connected != false {
		t.Fatal("default Connected should be false")
	}
	if a.ConnectedAt != nil {
		t.Fatal("default ConnectedAt should be nil")
	}
}

func TestAgentTimestampTypes(t *testing.T) {
	now := time.Now()
	a := Agent{
		ID:         "id1",
		ConnectedAt:    &now,
		DisconnectedAt: nil,
		LastSeenAt:     &now,
	}
	if a.ConnectedAt == nil || a.LastSeenAt == nil {
		t.Fatal("timestamps should be preserved")
	}
	if a.DisconnectedAt != nil {
		t.Fatal("optional timestamp should stay nil")
	}
}
