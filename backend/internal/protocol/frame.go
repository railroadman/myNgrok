package protocol

import (
	"encoding/binary"
	"fmt"
)

const (
	BinaryFrameVersion    byte = 1
	RequestBodyChunk      byte = 1
	ResponseBodyChunk     byte = 2
	binaryFrameHeader          = 12
	MaxBinaryFramePayload      = 32 << 10
)

// BinaryFrame carries one body chunk in a WebSocket binary message. Its wire
// layout is: version(1), type(1), requestIDLength(2), sequence(4),
// payloadLength(4), requestID, payload; all integer fields are big-endian.
type BinaryFrame struct {
	Type      byte
	RequestID string
	Sequence  uint32
	Payload   []byte
}

func (f BinaryFrame) MarshalBinary() ([]byte, error) {
	if f.Type != RequestBodyChunk && f.Type != ResponseBodyChunk {
		return nil, fmt.Errorf("unsupported binary frame type %d", f.Type)
	}
	if len(f.RequestID) == 0 || len(f.RequestID) > 1<<16-1 {
		return nil, fmt.Errorf("invalid request ID length %d", len(f.RequestID))
	}
	if len(f.Payload) > MaxBinaryFramePayload {
		return nil, fmt.Errorf("binary frame payload exceeds %d bytes", MaxBinaryFramePayload)
	}
	data := make([]byte, binaryFrameHeader+len(f.RequestID)+len(f.Payload))
	data[0], data[1] = BinaryFrameVersion, f.Type
	binary.BigEndian.PutUint16(data[2:4], uint16(len(f.RequestID)))
	binary.BigEndian.PutUint32(data[4:8], f.Sequence)
	binary.BigEndian.PutUint32(data[8:12], uint32(len(f.Payload)))
	copy(data[12:], f.RequestID)
	copy(data[12+len(f.RequestID):], f.Payload)
	return data, nil
}

func ParseBinaryFrame(data []byte) (BinaryFrame, error) {
	if len(data) < binaryFrameHeader || data[0] != BinaryFrameVersion {
		return BinaryFrame{}, fmt.Errorf("invalid binary frame")
	}
	frameType := data[1]
	if frameType != RequestBodyChunk && frameType != ResponseBodyChunk {
		return BinaryFrame{}, fmt.Errorf("unsupported binary frame type %d", frameType)
	}
	idLength := int(binary.BigEndian.Uint16(data[2:4]))
	payloadLength := int(binary.BigEndian.Uint32(data[8:12]))
	if payloadLength > MaxBinaryFramePayload {
		return BinaryFrame{}, fmt.Errorf("binary frame payload exceeds %d bytes", MaxBinaryFramePayload)
	}
	if idLength == 0 || len(data) != binaryFrameHeader+idLength+payloadLength {
		return BinaryFrame{}, fmt.Errorf("invalid binary frame length")
	}
	return BinaryFrame{Type: frameType, RequestID: string(data[12 : 12+idLength]), Sequence: binary.BigEndian.Uint32(data[4:8]), Payload: append([]byte(nil), data[12+idLength:]...)}, nil
}
