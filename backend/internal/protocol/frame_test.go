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

func TestBinaryFrameValidatesFrameTypeAndRequestID(t *testing.T) {
	for _, frame := range []BinaryFrame{
		{Type: 99, RequestID: "req_1"},
		{Type: RequestBodyChunk},
		{Type: RequestBodyChunk, RequestID: string(make([]byte, 1<<16))},
	} {
		if _, err := frame.MarshalBinary(); err == nil {
			t.Fatalf("invalid frame was accepted: %#v", frame)
		}
	}
}

func TestParseBinaryFrameRejectsInvalidTypesLengthsAndPayloads(t *testing.T) {
	valid, err := (BinaryFrame{Type: ResponseBodyChunk, RequestID: "req_1", Sequence: 1, Payload: []byte("ok")}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	cases := [][]byte{
		append([]byte{BinaryFrameVersion, 99}, valid[2:]...),
		append([]byte{2}, valid[1:]...),
		append([]byte{}, valid[:2]...),
		valid[:len(valid)-1],
	}
	oversized := append([]byte(nil), valid...)
	oversized[8], oversized[9], oversized[10], oversized[11] = 0, 0, 128, 1
	cases = append(cases, oversized)
	for _, data := range cases {
		if _, err := ParseBinaryFrame(data); err == nil {
			t.Fatalf("invalid frame was accepted: %v", data)
		}
	}
}
