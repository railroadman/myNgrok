CREATE TABLE traffic_totals (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    requests_total BIGINT NOT NULL DEFAULT 0,
    request_bytes BIGINT NOT NULL DEFAULT 0,
    response_bytes BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
