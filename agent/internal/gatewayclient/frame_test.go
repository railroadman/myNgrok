package gatewayclient

import (
	"encoding/binary"
	"testing"
)

func TestBinaryFrameRoundTrip(t *testing.T) {
	encoded, err := (binaryFrame{Type: responseBodyChunk, RequestID: "req_1", Sequence: 3, Payload: []byte("hello")}).marshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	frame, err := parseBinaryFrame(encoded)
	if err != nil || frame.Type != responseBodyChunk || frame.RequestID != "req_1" || frame.Sequence != 3 || string(frame.Payload) != "hello" {
		t.Fatalf("frame=%#v err=%v", frame, err)
	}
}

func TestBinaryFrameRejectsOversizedPayload(t *testing.T) {
	if _, err := (binaryFrame{Type: requestBodyChunk, RequestID: "req_1", Payload: make([]byte, maxBinaryFramePayload+1)}).marshalBinary(); err == nil {
		t.Fatal("oversized frame payload was accepted")
	}
}

func TestParseBinaryFrameRejectsOversizedDeclaredPayload(t *testing.T) {
	data := make([]byte, binaryFrameHeader)
	data[0], data[1] = binaryFrameVersion, requestBodyChunk
	binary.BigEndian.PutUint16(data[2:4], 1)
	binary.BigEndian.PutUint32(data[8:12], maxBinaryFramePayload+1)
	if _, err := parseBinaryFrame(data); err == nil {
		t.Fatal("oversized declared payload was accepted")
	}
}
