CREATE TABLE tunnels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    subdomain VARCHAR(63) NOT NULL UNIQUE,
    local_address VARCHAR(255) NOT NULL,
    protocol VARCHAR(16) NOT NULL DEFAULT 'http',
    status VARCHAR(16) NOT NULL DEFAULT 'closed' CHECK (status IN ('open', 'closed')),
    opened_at TIMESTAMPTZ NULL,
    closed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX tunnels_user_id_idx ON tunnels(user_id);
CREATE INDEX tunnels_agent_id_idx ON tunnels(agent_id);
CREATE INDEX tunnels_open_idx ON tunnels(status) WHERE status = 'open';
