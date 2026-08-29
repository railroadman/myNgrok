CREATE TABLE agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    agent_token_id UUID NOT NULL REFERENCES agent_tokens(id) ON DELETE RESTRICT,
    instance_id VARCHAR(255) NOT NULL,
    hostname VARCHAR(255) NOT NULL,
    os VARCHAR(64) NOT NULL,
    arch VARCHAR(64) NOT NULL,
    agent_version VARCHAR(64) NOT NULL,
    connected BOOLEAN NOT NULL DEFAULT FALSE,
    connected_at TIMESTAMPTZ NULL,
    disconnected_at TIMESTAMPTZ NULL,
    last_seen_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, instance_id)
);

CREATE INDEX agents_user_id_idx ON agents(user_id);
CREATE INDEX agents_connected_idx ON agents(connected) WHERE connected = TRUE;
