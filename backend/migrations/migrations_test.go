package migrations

import (
	"strings"
	"testing"
)

func TestAgentsMigrationIsRegisteredAndDefinesRequiredColumns(t *testing.T) {
	var sql string
	for _, migration := range All {
		if migration.Version == "004_create_agents" {
			sql = migration.UpSQL
			break
		}
	}
	if sql == "" {
		t.Fatal("agents migration is not registered")
	}
	for _, column := range []string{"user_id", "agent_token_id", "instance_id", "connected", "last_seen_at"} {
		if !strings.Contains(sql, column) {
			t.Errorf("agents migration is missing %q", column)
		}
	}
}

func TestTunnelsMigrationIsRegisteredAndDefinesRequiredColumns(t *testing.T) {
	var sql string
	for _, migration := range All { if migration.Version == "005_create_tunnels" { sql = migration.UpSQL; break } }
	if sql == "" { t.Fatal("tunnels migration is not registered") }
	for _, column := range []string{"user_id", "agent_id", "subdomain", "local_address", "status"} {
		if !strings.Contains(sql, column) { t.Errorf("tunnels migration is missing %q", column) }
	}
}
