package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/myngrok/backend/internal/agents"
	"github.com/myngrok/backend/internal/agenttokens"
	"github.com/myngrok/backend/internal/auth"
	"github.com/myngrok/backend/internal/config"
	"github.com/myngrok/backend/internal/database"
	"github.com/myngrok/backend/internal/gateway"
	"github.com/myngrok/backend/internal/protocol"
	"github.com/myngrok/backend/internal/server"
	"github.com/myngrok/backend/internal/tunnels"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	startupCtx, startupCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer startupCancel()
	db, err := database.Open(startupCtx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database unavailable", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := db.Migrate(startupCtx); err != nil {
		logger.Error("database migration failed", "error", err)
		os.Exit(1)
	}
	authService := auth.NewService(db.Raw(), cfg.Auth.AccessSecret, cfg.Auth.RefreshSecret, cfg.Auth.AccessTokenTTL, cfg.Auth.RefreshTokenTTL)
	authHandler := auth.NewHTTPHandler(authService, cfg.Environment != "development")
	agentService := agents.NewService(db.Raw())
	tunnelService := tunnels.NewService(db.Raw())
	registry := tunnels.NewRegistry()
	sessions := gateway.NewSessionManager()
	metrics := server.NewMetrics()
	metrics.SetGatewayCollector(func() server.GatewayMetrics {
		snapshot := sessions.MetricsSnapshot()
		return server.GatewayMetrics{ActiveSessions: snapshot.ActiveSessions, PendingRequests: snapshot.PendingRequests, ConnectionsTotal: snapshot.ConnectionsTotal, DisconnectionsTotal: snapshot.DisconnectionsTotal}
	})
	metrics.SetTunnelTrafficCollector(func() server.TunnelTrafficMetrics {
		traffic := registry.TrafficMetrics()
		return server.TunnelTrafficMetrics{RequestsTotal: traffic.RequestsTotal, RequestBytes: traffic.RequestBytes, ResponseBytes: traffic.ResponseBytes}
	})
	agentTokenHandler := agenttokens.NewHTTPHandler(agenttokens.NewService(db.Raw()), authService, sessions)
	agentConnectHandler := gateway.NewAgentConnectHandler(db.Raw(), sessions, agentService, func(ctx context.Context, sessionID, agentID string) {
		registry.CloseSession(sessionID)
		tunnelService.CloseAgentTunnels(ctx, agentID)
	}, func(ctx context.Context, sessionID, agentID, localAddress string) (protocol.TunnelOpenedPayload, error) {
		tunnel, err := tunnelService.ReopenForSession(ctx, agentID, localAddress)
		if err != nil {
			return protocol.TunnelOpenedPayload{}, err
		}
		registry.Open(tunnels.ActiveTunnel{ID: tunnel.ID, Subdomain: tunnel.Subdomain, AgentID: agentID, SessionID: sessionID, LocalAddress: localAddress})
		return protocol.TunnelOpenedPayload{TunnelID: tunnel.ID, Subdomain: tunnel.Subdomain, PublicURL: "https://" + tunnel.Subdomain + "." + cfg.PublicBaseDomain}, nil
	})
	agentHandler := agents.NewHTTPHandler(agentService, authService)
	tunnelHandler := tunnels.NewHTTPHandler(tunnelService, authService)
	handler := server.NewHandler(logger, func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		return db.Ping(ctx) == nil
	}, authHandler, agentTokenHandler, agentConnectHandler, agentHandler, tunnelHandler, tunnels.NewPublicHandlerWithTimeout(cfg.PublicBaseDomain, registry, sessions, cfg.TunnelRequestTimeout), cfg.CORSAllowedOrigins, metrics)
	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server listening", "address", cfg.HTTPAddr, "environment", cfg.Environment)
		errCh <- httpServer.ListenAndServe()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		sessions.CloseAll()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
			os.Exit(1)
		}
	}
}
