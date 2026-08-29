package gatewayclient

import "fmt"

// chunkAssembler reconstructs one binary body while rejecting chunks that are
// out of order or exceed the length announced by the request metadata.
type chunkAssembler struct {
	expected int64
	next     uint32
	body     []byte
}

func newChunkAssembler(expected int64) (*chunkAssembler, error) {
	if expected < -1 {
		return nil, fmt.Errorf("invalid content length")
	}
	capacity := int64(0)
	if expected > 0 {
		capacity = expected
	}
	return &chunkAssembler{expected: expected, body: make([]byte, 0, capacity)}, nil
}

func (a *chunkAssembler) add(frame binaryFrame) (bool, error) {
	if frame.Type != requestBodyChunk || frame.Sequence != a.next {
		return false, fmt.Errorf("unexpected request body chunk")
	}
	if a.expected >= 0 && int64(len(a.body)+len(frame.Payload)) > a.expected {
		return false, fmt.Errorf("request body exceeds content length")
	}
	a.body = append(a.body, frame.Payload...)
	a.next++
	return a.expected >= 0 && int64(len(a.body)) == a.expected, nil
}

func (a *chunkAssembler) complete() error {
	if a.expected >= 0 && int64(len(a.body)) != a.expected {
		return fmt.Errorf("request body is shorter than content length")
	}
	return nil
}
