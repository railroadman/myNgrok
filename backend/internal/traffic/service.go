package traffic

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Metrics struct {
	RequestsTotal uint64
	RequestBytes  uint64
	ResponseBytes uint64
}

type Totals struct {
	RequestsTotal uint64 `json:"requestsTotal"`
	RequestBytes  uint64 `json:"requestBytes"`
	ResponseBytes uint64 `json:"responseBytes"`
}

type Service struct{ pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool} }

// AddDelta accumulates traffic counters for a user. It is meant to be called
// periodically with the delta since the last call, not with a running total.
func (s *Service) AddDelta(ctx context.Context, userID string, delta Metrics) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO traffic_totals (user_id, requests_total, request_bytes, response_bytes, updated_at)
		VALUES ($1,$2,$3,$4,NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			requests_total = traffic_totals.requests_total + EXCLUDED.requests_total,
			request_bytes  = traffic_totals.request_bytes  + EXCLUDED.request_bytes,
			response_bytes = traffic_totals.response_bytes + EXCLUDED.response_bytes,
			updated_at = NOW()
	`, userID, delta.RequestsTotal, delta.RequestBytes, delta.ResponseBytes)
	if err != nil {
		return fmt.Errorf("add traffic delta: %w", err)
	}
	return nil
}

func (s *Service) GetTotals(ctx context.Context, userID string) (Totals, error) {
	var totals Totals
	err := s.pool.QueryRow(ctx, `SELECT requests_total,request_bytes,response_bytes FROM traffic_totals WHERE user_id=$1`, userID).
		Scan(&totals.RequestsTotal, &totals.RequestBytes, &totals.ResponseBytes)
	if err == nil {
		return totals, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return Totals{}, nil
	}
	return Totals{}, fmt.Errorf("get traffic totals: %w", err)
}
