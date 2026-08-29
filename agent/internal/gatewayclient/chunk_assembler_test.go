package gatewayclient

import (
	"context"
	"strings"
	"testing"
)

func TestChunkAssemblerReconstructsOrderedBody(t *testing.T) {
	a, err := newChunkAssembler(5)
	if err != nil {
		t.Fatal(err)
	}
	if done, err := a.add(binaryFrame{Type: requestBodyChunk, Sequence: 0, Payload: []byte("he")}); err != nil || done {
		t.Fatalf("done=%v err=%v", done, err)
	}
	if done, err := a.add(binaryFrame{Type: requestBodyChunk, Sequence: 1, Payload: []byte("llo")}); err != nil || !done || string(a.body) != "hello" {
		t.Fatalf("done=%v body=%q err=%v", done, a.body, err)
	}
}

func TestChunkAssemblerRejectsInvalidSequence(t *testing.T) {
	a, _ := newChunkAssembler(1)
	if _, err := a.add(binaryFrame{Type: requestBodyChunk, Sequence: 1, Payload: []byte("x")}); err == nil {
		t.Fatal("expected error")
	}
}

func TestClientRejectsExcessiveConcurrency(t *testing.T) {
	err := New().Connect(context.Background(), Config{GatewayURL: "ws://example.test", Token: "tkn_test", MaxConcurrentRequests: maxConcurrentRequests + 1}, nil)
	if err == nil || !strings.Contains(err.Error(), "max concurrent requests") {
		t.Fatalf("error=%v", err)
	}
}

func TestChunkAssemblerRejectsIncompleteKnownBody(t *testing.T) {
	assembler, err := newChunkAssembler(2)
	if err != nil {
		t.Fatal(err)
	}
	if err := assembler.complete(); err == nil {
		t.Fatal("incomplete body was accepted")
	}
}
