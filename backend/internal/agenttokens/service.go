package agenttokens

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Token struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	ExpiresAt  *time.Time `json:"expiresAt"`
	RevokedAt  *time.Time `json:"revokedAt"`
}

type CreatedToken struct {
	Token
	Plaintext string
}
type Service struct{ pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

func (s *Service) Create(ctx context.Context, userID, name string) (CreatedToken, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 128 {
		return CreatedToken{}, fmt.Errorf("token name must be 1-128 characters")
	}
	raw, err := generate()
	if err != nil {
		return CreatedToken{}, err
	}
	result := CreatedToken{Plaintext: raw}
	err = s.pool.QueryRow(ctx, `INSERT INTO agent_tokens (user_id,name,token_prefix,token_hash) VALUES ($1,$2,$3,$4) RETURNING id::text,name,token_prefix,created_at`, userID, name, visiblePrefix(raw), hash(raw)).Scan(&result.ID, &result.Name, &result.Prefix, &result.CreatedAt)
	if err != nil {
		return CreatedToken{}, fmt.Errorf("create agent token: %w", err)
	}
	return result, nil
}

func (s *Service) List(ctx context.Context, userID string) ([]Token, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text,name,token_prefix,created_at,last_used_at,expires_at,revoked_at FROM agent_tokens WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list agent tokens: %w", err)
	}
	defer rows.Close()
	items := make([]Token, 0)
	for rows.Next() {
		var item Token
		if err := rows.Scan(&item.ID, &item.Name, &item.Prefix, &item.CreatedAt, &item.LastUsedAt, &item.ExpiresAt, &item.RevokedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Service) Revoke(ctx context.Context, userID, tokenID string) (string, bool, error) {
	var tokenHash string
	err := s.pool.QueryRow(ctx, `UPDATE agent_tokens SET revoked_at=NOW() WHERE id=$1 AND user_id=$2 AND revoked_at IS NULL RETURNING token_hash`, tokenID, userID).Scan(&tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("revoke agent token: %w", err)
	}
	return tokenHash, true, nil
}
func generate() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return "tkn_" + base64.RawURLEncoding.EncodeToString(b), nil
}
func hash(raw string) string { sum := sha256.Sum256([]byte(raw)); return hex.EncodeToString(sum[:]) }
func visiblePrefix(raw string) string {
	if len(raw) <= 12 {
		return raw
	}
	return raw[:12]
}
