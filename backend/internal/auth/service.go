package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailTaken         = errors.New("email is already registered")
	ErrUnauthorized       = errors.New("unauthorized")
)

type User struct{ ID, Email string }
type Tokens struct {
	AccessToken, RefreshToken string
	RefreshExpiresAt          time.Time
}
type Service struct {
	pool                        *pgxpool.Pool
	accessSecret, refreshSecret []byte
	accessTTL, refreshTTL       time.Duration
}

func NewService(pool *pgxpool.Pool, accessSecret, refreshSecret string, accessTTL, refreshTTL time.Duration) *Service {
	return &Service{pool: pool, accessSecret: []byte(accessSecret), refreshSecret: []byte(refreshSecret), accessTTL: accessTTL, refreshTTL: refreshTTL}
}

func (s *Service) Register(ctx context.Context, email, password string) (User, error) {
	email = normalizeEmail(email)
	if !validEmail(email) || len(password) < 10 {
		return User{}, fmt.Errorf("invalid registration data")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}
	var user User
	err = s.pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id::text, email`, email, string(hash)).Scan(&user.ID, &user.Email)
	if err != nil {
		if strings.Contains(err.Error(), "users_email_key") {
			return User{}, ErrEmailTaken
		}
		return User{}, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

func (s *Service) Login(ctx context.Context, email, password, userAgent, ip string) (User, Tokens, error) {
	email = normalizeEmail(email)
	var user User
	var hash string
	var status string
	err := s.pool.QueryRow(ctx, `SELECT id::text, email, password_hash, status FROM users WHERE email=$1`, email).Scan(&user.ID, &user.Email, &hash, &status)
	if err != nil || status != "active" || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return User{}, Tokens{}, ErrInvalidCredentials
	}
	if _, err := s.pool.Exec(ctx, `UPDATE users SET last_login_at=NOW(), updated_at=NOW() WHERE id=$1`, user.ID); err != nil {
		return User{}, Tokens{}, fmt.Errorf("update login: %w", err)
	}
	tokens, err := s.issue(ctx, user.ID, userAgent, ip)
	if err != nil {
		return User{}, Tokens{}, err
	}
	return user, tokens, nil
}

func (s *Service) Refresh(ctx context.Context, raw, userAgent, ip string) (User, Tokens, error) {
	if raw == "" {
		return User{}, Tokens{}, ErrUnauthorized
	}
	hash := hashToken(raw, s.refreshSecret)
	var user User
	err := s.pool.QueryRow(ctx, `SELECT u.id::text, u.email FROM refresh_tokens r JOIN users u ON u.id=r.user_id WHERE r.token_hash=$1 AND r.revoked_at IS NULL AND r.expires_at>NOW() AND u.status='active'`, hash).Scan(&user.ID, &user.Email)
	if err != nil {
		return User{}, Tokens{}, ErrUnauthorized
	}
	if _, err := s.pool.Exec(ctx, `UPDATE refresh_tokens SET revoked_at=NOW() WHERE token_hash=$1`, hash); err != nil {
		return User{}, Tokens{}, fmt.Errorf("revoke used token: %w", err)
	}
	tokens, err := s.issue(ctx, user.ID, userAgent, ip)
	return user, tokens, err
}

func (s *Service) Logout(ctx context.Context, raw string) error {
	if raw == "" {
		return nil
	}
	_, err := s.pool.Exec(ctx, `UPDATE refresh_tokens SET revoked_at=NOW() WHERE token_hash=$1 AND revoked_at IS NULL`, hashToken(raw, s.refreshSecret))
	return err
}

func (s *Service) UserFromAccessToken(ctx context.Context, token string) (User, error) {
	claims, err := verifyJWT(token, s.accessSecret)
	if err != nil || claims.Type != "access" || claims.Subject == "" || claims.ExpiresAt < time.Now().Unix() {
		return User{}, ErrUnauthorized
	}
	var user User
	err = s.pool.QueryRow(ctx, `SELECT id::text, email FROM users WHERE id=$1 AND status='active'`, claims.Subject).Scan(&user.ID, &user.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUnauthorized
	}
	if err != nil {
		return User{}, fmt.Errorf("load current user: %w", err)
	}
	return user, nil
}

func (s *Service) issue(ctx context.Context, userID, userAgent, ip string) (Tokens, error) {
	access, err := signJWT(claims{Subject: userID, Type: "access", IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(s.accessTTL).Unix()}, s.accessSecret)
	if err != nil {
		return Tokens{}, err
	}
	refreshRaw, err := randomToken()
	if err != nil {
		return Tokens{}, err
	}
	expires := time.Now().Add(s.refreshTTL)
	if _, err := s.pool.Exec(ctx, `INSERT INTO refresh_tokens (user_id, token_hash, expires_at, user_agent, ip_address) VALUES ($1,$2,$3,$4,NULLIF($5,'')::inet)`, userID, hashToken(refreshRaw, s.refreshSecret), expires, userAgent, ip); err != nil {
		return Tokens{}, fmt.Errorf("store refresh token: %w", err)
	}
	return Tokens{AccessToken: access, RefreshToken: refreshRaw, RefreshExpiresAt: expires}, nil
}

func normalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }
func validEmail(email string) bool {
	return len(email) <= 320 && strings.Count(email, "@") == 1 && !strings.HasPrefix(email, "@") && !strings.HasSuffix(email, "@")
}
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func hashToken(raw string, secret []byte) string {
	h := hmac.New(sha256.New, secret)
	_, _ = h.Write([]byte(raw))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

type claims struct {
	Subject   string `json:"sub"`
	Type      string `json:"type"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

func signJWT(c claims, secret []byte) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	encoded := header + "." + base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
func verifyJWT(raw string, secret []byte) (claims, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return claims{}, ErrUnauthorized
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(mac.Sum(nil), sig) {
		return claims{}, ErrUnauthorized
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims{}, ErrUnauthorized
	}
	var c claims
	if err = json.Unmarshal(data, &c); err != nil {
		return claims{}, ErrUnauthorized
	}
	return c, nil
}
