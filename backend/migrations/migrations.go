package migrations

import _ "embed"

type Migration struct {
	Version string
	UpSQL   string
}

//go:embed 001_enable_pgcrypto.up.sql
var enablePGCrypto string

//go:embed 002_create_auth.up.sql
var createAuth string

//go:embed 003_create_agent_tokens.up.sql
var createAgentTokens string

//go:embed 004_create_agents.up.sql
var createAgents string

//go:embed 005_create_tunnels.up.sql
var createTunnels string

//go:embed 006_create_traffic_totals.up.sql
var createTrafficTotals string

var All = []Migration{
	{Version: "001_enable_pgcrypto", UpSQL: enablePGCrypto},
	{Version: "002_create_auth", UpSQL: createAuth},
	{Version: "003_create_agent_tokens", UpSQL: createAgentTokens},
	{Version: "004_create_agents", UpSQL: createAgents},
	{Version: "005_create_tunnels", UpSQL: createTunnels},
	{Version: "006_create_traffic_totals", UpSQL: createTrafficTotals},
}
