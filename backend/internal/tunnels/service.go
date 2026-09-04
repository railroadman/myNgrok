package tunnels

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type Tunnel struct {
	ID           string     `json:"id"`
	Subdomain    string     `json:"subdomain"`
	LocalAddress string     `json:"localAddress"`
	Status       string     `json:"status"`
	OpenedAt     *time.Time `json:"openedAt"`
	ClosedAt     *time.Time `json:"closedAt"`
	AgentID      string     `json:"agentId"`
	UserID       string     `json:"-"`
}
type Service struct{ pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool} }

// CloseAgentTunnels marks every currently open tunnel for an agent as closed.
// Runtime routes are removed separately by Registry.CloseSession.
func (s *Service) CloseAgentTunnels(ctx context.Context, agentID string) {
	if agentID == "" {
		return
	}
	_, _ = s.pool.Exec(ctx, `UPDATE tunnels SET status='closed',closed_at=NOW(),updated_at=NOW() WHERE agent_id=$1 AND status='open'`, agentID)
}

// ReopenForSession restores the most recently closed tunnel with the same
// local destination. New destinations receive a fresh public subdomain.
func (s *Service) ReopenForSession(ctx context.Context, agentID, localAddress string) (Tunnel, error) {
	var tunnel Tunnel
	err := s.pool.QueryRow(ctx, `UPDATE tunnels SET status='open',opened_at=NOW(),closed_at=NULL,updated_at=NOW() WHERE id=(SELECT id FROM tunnels WHERE agent_id=$1 AND local_address=$2 AND status='closed' ORDER BY updated_at DESC LIMIT 1) RETURNING id::text,subdomain,local_address,status,opened_at,closed_at,agent_id::text,user_id::text`, agentID, localAddress).Scan(&tunnel.ID, &tunnel.Subdomain, &tunnel.LocalAddress, &tunnel.Status, &tunnel.OpenedAt, &tunnel.ClosedAt, &tunnel.AgentID, &tunnel.UserID)
	if err == nil {
		return tunnel, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Tunnel{}, fmt.Errorf("reopen tunnel: %w", err)
	}
	subdomain, err := GenerateSubdomain()
	if err != nil {
		return Tunnel{}, err
	}
	err = s.pool.QueryRow(ctx, `INSERT INTO tunnels (user_id,agent_id,subdomain,local_address,status,opened_at) SELECT user_id,$1,$2,$3,'open',NOW() FROM agents WHERE id=$1 RETURNING id::text,subdomain,local_address,status,opened_at,closed_at,agent_id::text,user_id::text`, agentID, subdomain, localAddress).Scan(&tunnel.ID, &tunnel.Subdomain, &tunnel.LocalAddress, &tunnel.Status, &tunnel.OpenedAt, &tunnel.ClosedAt, &tunnel.AgentID, &tunnel.UserID)
	if err != nil {
		return Tunnel{}, fmt.Errorf("create tunnel: %w", err)
	}
	return tunnel, nil
}

func (s *Service) List(ctx context.Context, userID string) ([]Tunnel, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text,subdomain,local_address,status,opened_at,closed_at,agent_id::text FROM tunnels WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list tunnels: %w", err)
	}
	defer rows.Close()
	items := make([]Tunnel, 0)
	for rows.Next() {
		var item Tunnel
		if err := rows.Scan(&item.ID, &item.Subdomain, &item.LocalAddress, &item.Status, &item.OpenedAt, &item.ClosedAt, &item.AgentID); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
