package agents

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/myngrok/backend/internal/protocol"
)

type Agent struct {
	ID             string     `json:"id"`
	InstanceID     string     `json:"instanceID"`
	Hostname       string     `json:"hostname"`
	OS             string     `json:"os"`
	Arch           string     `json:"arch"`
	Version        string     `json:"version"`
	Connected      bool       `json:"connected"`
	ConnectedAt    *time.Time `json:"connectedAt"`
	DisconnectedAt *time.Time `json:"disconnectedAt"`
	LastSeenAt     *time.Time `json:"lastSeenAt"`
}

type Service struct{ pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

func (s *Service) Connect(ctx context.Context, rawToken string, hello protocol.ClientHelloPayload) (string, error) {
	if strings.TrimSpace(hello.InstanceID) == "" || strings.TrimSpace(hello.Hostname) == "" {
		return "", fmt.Errorf("agent instance ID and hostname are required")
	}
	sum := sha256.Sum256([]byte(rawToken))
	var tokenID, userID string
	err := s.pool.QueryRow(ctx, `SELECT id::text,user_id::text FROM agent_tokens WHERE token_hash=$1 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at>NOW())`, hex.EncodeToString(sum[:])).Scan(&tokenID, &userID)
	if err != nil {
		return "", fmt.Errorf("find agent token: %w", err)
	}
	var agentID string
	err = s.pool.QueryRow(ctx, `INSERT INTO agents (user_id,agent_token_id,instance_id,hostname,os,arch,agent_version,connected,connected_at,last_seen_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,TRUE,NOW(),NOW())
ON CONFLICT (user_id,instance_id) DO UPDATE SET agent_token_id=EXCLUDED.agent_token_id,hostname=EXCLUDED.hostname,os=EXCLUDED.os,arch=EXCLUDED.arch,agent_version=EXCLUDED.agent_version,connected=TRUE,connected_at=NOW(),disconnected_at=NULL,last_seen_at=NOW(),updated_at=NOW()
RETURNING id::text`, userID, tokenID, hello.InstanceID, hello.Hostname, hello.OS, hello.Arch, hello.AgentVersion).Scan(&agentID)
	if err != nil {
		return "", fmt.Errorf("upsert agent: %w", err)
	}
	_, _ = s.pool.Exec(ctx, `UPDATE agent_tokens SET last_used_at=NOW() WHERE id=$1`, tokenID)
	return agentID, nil
}

func (s *Service) Touch(ctx context.Context, id string) {
	_, _ = s.pool.Exec(ctx, `UPDATE agents SET last_seen_at=NOW(),updated_at=NOW() WHERE id=$1 AND connected=TRUE`, id)
}
func (s *Service) Disconnect(ctx context.Context, id string) {
	_, _ = s.pool.Exec(ctx, `UPDATE agents SET connected=FALSE,disconnected_at=NOW(),updated_at=NOW() WHERE id=$1`, id)
}

func (s *Service) List(ctx context.Context, userID string) ([]Agent, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text,instance_id,hostname,os,arch,agent_version,connected,connected_at,disconnected_at,last_seen_at FROM agents WHERE user_id=$1 ORDER BY last_seen_at DESC NULLS LAST`, userID)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer rows.Close()
	items := make([]Agent, 0)
	for rows.Next() {
		var a Agent
		if err := rows.Scan(&a.ID, &a.InstanceID, &a.Hostname, &a.OS, &a.Arch, &a.Version, &a.Connected, &a.ConnectedAt, &a.DisconnectedAt, &a.LastSeenAt); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, rows.Err()
}
