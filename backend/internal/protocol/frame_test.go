package protocol

import "testing"

func TestBinaryFrameRoundTrip(t *testing.T) {
	encoded, err := (BinaryFrame{Type: RequestBodyChunk, RequestID: "req_1", Sequence: 3, Payload: []byte("hello")}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	frame, err := ParseBinaryFrame(encoded)
	if err != nil || frame.Type != RequestBodyChunk || frame.RequestID != "req_1" || frame.Sequence != 3 || string(frame.Payload) != "hello" {
		t.Fatalf("frame=%#v err=%v", frame, err)
	}
}

func TestParseBinaryFrameRejectsInvalidData(t *testing.T) {
	if _, err := ParseBinaryFrame([]byte{1, 1}); err == nil {
		t.Fatal("expected error")
	}
}

func TestBinaryFrameRejectsOversizedPayload(t *testing.T) {
	if _, err := (BinaryFrame{Type: RequestBodyChunk, RequestID: "req_1", Payload: make([]byte, MaxBinaryFramePayload+1)}).MarshalBinary(); err == nil {
		t.Fatal("oversized frame payload was accepted")
	}
}
