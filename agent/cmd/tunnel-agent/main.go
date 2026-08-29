package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/myngrok/agent/internal/gatewayclient"
)

var version = "0.1.0"

var exitProcess = os.Exit

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	exitProcess(run(os.Args, ctx, gatewayclient.New(), os.Stdout, os.Stderr))
}

func run(args []string, ctx context.Context, client gatewayRunner, output, errorOutput io.Writer) int {
	if len(args) < 2 {
		usage(errorOutput)
		return 2
	}
	switch args[1] {
	case "version":
		fmt.Fprintf(output, "tunnel-agent %s\n", version)
		return 0
	case "http":
		if err := runHTTP(ctx, args[2:], client, output, errorOutput); err != nil {
			fmt.Fprintln(errorOutput, err)
			return 2
		}
		return 0
	default:
		usage(errorOutput)
		return 2
	}
}

type gatewayRunner interface {
	Run(context.Context, gatewayclient.Config, func(gatewayclient.Connected), func(error, time.Duration)) error
}

func runHTTP(ctx context.Context, args []string, client gatewayRunner, output, errorOutput io.Writer) error {
	if client == nil {
		return fmt.Errorf("gateway client is required")
	}
	flags := flag.NewFlagSet("http", flag.ContinueOnError)
	flags.SetOutput(errorOutput)
	token := flags.String("token", "", "agent token")
	gateway := flags.String("gateway", "ws://localhost:8080/api/v1/agent/connect", "gateway WebSocket URL")
	localAddress := ""
	// The standard flag package stops parsing at the first positional argument.
	// Accept the conventional `http <address> --token ...` form by moving that
	// address aside before parsing the remaining flags.
	if len(args) > 0 && args[0] != "" && args[0][0] != '-' {
		localAddress = args[0]
		args = args[1:]
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if localAddress == "" {
		if flags.NArg() != 1 {
			return fmt.Errorf("http requires a local port or host:port")
		}
		localAddress = flags.Arg(0)
	} else if flags.NArg() != 0 {
		return fmt.Errorf("http accepts exactly one local port or host:port")
	}
	if *token == "" {
		return fmt.Errorf("--token is required until login/config support is implemented")
	}
	fmt.Fprintf(output, "Tunnel Agent %s\n", version)
	fmt.Fprintf(output, "Local destination: %s\n", localAddress)
	fmt.Fprintf(output, "Gateway: %s\n", *gateway)
	var sessionMetrics gatewayclient.SessionMetrics
	if err := client.Run(ctx, gatewayclient.Config{GatewayURL: *gateway, Token: *token, Version: version, LocalAddress: localAddress, OnSessionMetrics: func(metrics gatewayclient.SessionMetrics) {
		sessionMetrics = metrics
	}}, func(connection gatewayclient.Connected) {
		fmt.Fprintln(output, "Connected to gateway")
		fmt.Fprintf(output, "Session: %s\n", connection.SessionID)
	}, func(err error, retryIn time.Duration) {
		fmt.Fprintf(errorOutput, "Connection lost (%v); reconnecting in %s\n", err, retryIn)
	}); err != nil {
		return fmt.Errorf("gateway connection failed: %w", err)
	}
	fmt.Fprintf(output, "Session metrics: connections=%d disconnections=%d\n", sessionMetrics.ConnectionsTotal, sessionMetrics.DisconnectionsTotal)
	return nil
}

func usage(writer io.Writer) {
	fmt.Fprintln(writer, "Usage: tunnel-agent <command> [options]")
	fmt.Fprintln(writer, "Commands: version, http")
}
