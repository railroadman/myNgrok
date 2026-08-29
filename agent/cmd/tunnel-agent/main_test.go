package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/myngrok/agent/internal/gatewayclient"
)

type fakeGatewayRunner struct{ err error }

func (f fakeGatewayRunner) Run(_ context.Context, config gatewayclient.Config, connected func(gatewayclient.Connected), _ func(error, time.Duration)) error {
	if connected != nil {
		connected(gatewayclient.Connected{SessionID: "ses_test"})
	}
	if config.OnSessionMetrics != nil {
		config.OnSessionMetrics(gatewayclient.SessionMetrics{ConnectionsTotal: 1, DisconnectionsTotal: 1})
	}
	return f.err
}

func TestUsage(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	usage(&output)
	if output.Len() == 0 {
		t.Fatal("usage must write help text")
	}
}

func TestRunHTTPValidatesArgumentsAndRunsClient(t *testing.T) {
	var output, errorsOutput bytes.Buffer
	if err := runHTTP(context.Background(), []string{"8080"}, fakeGatewayRunner{}, &output, &errorsOutput); err == nil {
		t.Fatal("missing token was accepted")
	}
	if err := runHTTP(context.Background(), []string{"--token", "tkn_test", "8080"}, fakeGatewayRunner{}, &output, &errorsOutput); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Local destination: 8080", "Connected to gateway", "Session: ses_test", "connections=1 disconnections=1"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("missing %q in %q", expected, output.String())
		}
	}
	if err := runHTTP(context.Background(), []string{"--token", "tkn_test", "8080"}, fakeGatewayRunner{err: errors.New("offline")}, &output, &errorsOutput); err == nil {
		t.Fatal("client error was not returned")
	}
}

func TestRunDispatchesCommands(t *testing.T) {
	var output, errorsOutput bytes.Buffer
	if code := run([]string{"tunnel-agent", "version"}, context.Background(), fakeGatewayRunner{}, &output, &errorsOutput); code != 0 || !strings.Contains(output.String(), "tunnel-agent") {
		t.Fatalf("version code=%d output=%q", code, output.String())
	}
	if code := run([]string{"tunnel-agent", "unknown"}, context.Background(), fakeGatewayRunner{}, &output, &errorsOutput); code != 2 || !strings.Contains(errorsOutput.String(), "Usage:") {
		t.Fatalf("unknown code=%d errors=%q", code, errorsOutput.String())
	}
	if code := run([]string{"tunnel-agent", "http", "--token", "tkn_test", "8080"}, context.Background(), fakeGatewayRunner{}, &output, &errorsOutput); code != 0 {
		t.Fatalf("http code=%d errors=%q", code, errorsOutput.String())
	}
	if code := run([]string{"tunnel-agent", "http", "8080", "--token", "tkn_test", "--gateway", "ws://localhost:8080/api/v1/agent/connect"}, context.Background(), fakeGatewayRunner{}, &output, &errorsOutput); code != 0 {
		t.Fatalf("address-first http code=%d errors=%q", code, errorsOutput.String())
	}
}

func TestMainDelegatesToRunner(t *testing.T) {
	originalArgs, originalExit := os.Args, exitProcess
	defer func() { os.Args, exitProcess = originalArgs, originalExit }()
	os.Args = []string{"tunnel-agent", "version"}
	code := -1
	exitProcess = func(value int) { code = value }
	main()
	if code != 0 {
		t.Fatalf("exit code=%d", code)
	}
}
