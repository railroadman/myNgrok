package gateway

import (
	"context"
	"testing"
	"time"
)

func TestSessionManagerCorrelatesOutOfOrderResponsesByRequestID(t *testing.T) {
	manager := NewSessionManager()
	first := manager.RegisterRequest("agent-1", "req_first")
	second := manager.RegisterRequest("agent-1", "req_second")
	if !manager.DeliverResponse("req_second", Response{StatusCode: 202, Body: []byte("second")}) {
		t.Fatal("second response was not delivered")
	}
	if !manager.DeliverResponse("req_first", Response{StatusCode: 201, Body: []byte("first")}) {
		t.Fatal("first response was not delivered")
	}
	if response := <-first; response.StatusCode != 201 || string(response.Body) != "first" {
		t.Fatalf("first response = %#v", response)
	}
	if response := <-second; response.StatusCode != 202 || string(response.Body) != "second" {
		t.Fatalf("second response = %#v", response)
	}
	if manager.DeliverResponse("req_first", Response{}) {
		t.Fatal("completed request accepted a duplicate response")
	}
}

func TestSessionManagerCancelsOnlyDisconnectedSessionRequests(t *testing.T) {
	manager := NewSessionManager()
	manager.Register(Session{ID: "agent-1"})
	manager.Register(Session{ID: "agent-2"})
	first := manager.RegisterRequest("agent-1", "req_first")
	second := manager.RegisterRequest("agent-2", "req_second")

	manager.Remove("agent-1")
	select {
	case response := <-first:
		if response.Error != "tunnel agent disconnected" {
			t.Fatalf("disconnect response = %#v", response)
		}
	case <-time.After(time.Second):
		t.Fatal("disconnected session request was not cancelled")
	}
	select {
	case <-second:
		t.Fatal("request for another session was cancelled")
	default:
	}
	if !manager.DeliverResponse("req_second", Response{StatusCode: 204}) {
		t.Fatal("active session response was rejected")
	}
}

func TestSessionManagerTracksSessionLifecycle(t *testing.T) {
	manager := NewSessionManager()
	manager.Register(Session{ID: "agent-1", AgentTokenHash: "token-1"})

	if got := manager.Count(); got != 1 {
		t.Fatalf("Count() = %d, want 1", got)
	}
	if !manager.Touch("agent-1") {
		t.Fatal("Touch() = false, want true")
	}
	if manager.Touch("unknown") {
		t.Fatal("Touch() = true for unknown session")
	}

	manager.Remove("agent-1")
	if got := manager.Count(); got != 0 {
		t.Fatalf("Count() = %d after Remove(), want 0", got)
	}
}

func TestSessionManagerSetsLastSeen(t *testing.T) {
	manager := NewSessionManager()
	manager.Register(Session{ID: "agent-1"})
	if !manager.Touch("agent-1") {
		t.Fatal("registered session must be touchable")
	}
}

func TestSessionManagerQueuesMessageForSession(t *testing.T) {
	manager := NewSessionManager()
	outgoing := make(chan OutboundMessage, 1)
	manager.Register(Session{ID: "agent-1", Outbound: outgoing})
	if !manager.Send("agent-1", []byte("message")) {
		t.Fatal("Send() = false")
	}
	if got := string((<-outgoing).Data); got != "message" {
		t.Fatalf("message = %q", got)
	}
	if manager.Send("missing", []byte("message")) {
		t.Fatal("unknown session accepted message")
	}
}

func TestSessionManagerRejectsMessageWhenBoundedQueueIsFull(t *testing.T) {
	manager := NewSessionManager()
	outgoing := make(chan OutboundMessage, 1)
	manager.Register(Session{ID: "agent-1", Outbound: outgoing})
	if !manager.Send("agent-1", []byte("first")) {
		t.Fatal("first message was rejected")
	}
	if manager.SendBinary("agent-1", []byte("second")) {
		t.Fatal("full outbound queue accepted a message")
	}
}

func TestSessionManagerContextSendWaitsForQueueCapacity(t *testing.T) {
	manager := NewSessionManager()
	outgoing := make(chan OutboundMessage, 1)
	manager.Register(Session{ID: "agent-1", Outbound: outgoing})
	if !manager.Send("agent-1", []byte("first")) {
		t.Fatal("first message was rejected")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() {
		time.Sleep(20 * time.Millisecond)
		<-outgoing
	}()
	if !manager.SendBinaryContext(ctx, "agent-1", []byte("second")) {
		t.Fatal("context send did not wait for freed queue capacity")
	}
	if got := string((<-outgoing).Data); got != "second" {
		t.Fatalf("message=%q", got)
	}
}

func TestSessionManagerContextSendStopsOnCancellation(t *testing.T) {
	manager := NewSessionManager()
	outgoing := make(chan OutboundMessage, 1)
	manager.Register(Session{ID: "agent-1", Outbound: outgoing})
	_ = manager.Send("agent-1", []byte("first"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if manager.SendContext(ctx, "agent-1", []byte("second")) {
		t.Fatal("cancelled context accepted a queued message")
	}
}

func TestSessionManagerCloseAllCancelsActiveSessions(t *testing.T) {
	manager := NewSessionManager()
	first, cancelFirst := context.WithCancel(context.Background())
	second, cancelSecond := context.WithCancel(context.Background())
	defer cancelFirst()
	defer cancelSecond()
	manager.Register(Session{ID: "agent-1", Cancel: cancelFirst})
	manager.Register(Session{ID: "agent-2", Cancel: cancelSecond})
	manager.CloseAll()
	select {
	case <-first.Done():
	case <-time.After(time.Second):
		t.Fatal("first session was not cancelled")
	}
	select {
	case <-second.Done():
	case <-time.After(time.Second):
		t.Fatal("second session was not cancelled")
	}
}

func TestSessionManagerDisconnectsOnlySessionsWithRevokedToken(t *testing.T) {
	manager := NewSessionManager()
	first, cancelFirst := context.WithCancel(context.Background())
	second, cancelSecond := context.WithCancel(context.Background())
	defer cancelFirst()
	defer cancelSecond()
	manager.Register(Session{ID: "agent-1", AgentTokenHash: "hash-revoked", Cancel: cancelFirst})
	manager.Register(Session{ID: "agent-2", AgentTokenHash: "hash-active", Cancel: cancelSecond})
	manager.DisconnectTokenHash("hash-revoked")
	select {
	case <-first.Done():
	case <-time.After(time.Second):
		t.Fatal("revoked token session was not cancelled")
	}
	select {
	case <-second.Done():
		t.Fatal("active token session was cancelled")
	default:
	}
}

func TestSessionManagerMetricsSnapshot(t *testing.T) {
	manager := NewSessionManager()
	manager.Register(Session{ID: "agent-1"})
	manager.RegisterRequest("agent-1", "req_1")
	manager.Remove("agent-1")
	snapshot := manager.MetricsSnapshot()
	if snapshot.ActiveSessions != 0 || snapshot.PendingRequests != 0 || snapshot.ConnectionsTotal != 1 || snapshot.DisconnectionsTotal != 1 {
		t.Fatalf("snapshot=%#v", snapshot)
	}
}

func TestSessionManagerCancelsExplicitRequestAndFindsAgent(t *testing.T) {
	manager := NewSessionManager()
	manager.Register(Session{ID: "agent-1", AgentID: "agent-db-id"})
	if agentID, ok := manager.AgentID("agent-1"); !ok || agentID != "agent-db-id" {
		t.Fatalf("AgentID() = %q, %v", agentID, ok)
	}
	manager.RegisterRequest("agent-1", "req_cancel")
	manager.CancelRequest("req_cancel")
	if manager.DeliverResponse("req_cancel", Response{}) {
		t.Fatal("cancelled request accepted a response")
	}
}
