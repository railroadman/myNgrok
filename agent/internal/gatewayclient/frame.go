package gatewayclient

import (
	"encoding/binary"
	"fmt"
)

const (
	binaryFrameVersion    byte = 1
	requestBodyChunk      byte = 1
	responseBodyChunk     byte = 2
	binaryFrameHeader          = 12
	maxBinaryFramePayload      = 32 << 10
)

type binaryFrame struct {
	Type      byte
	RequestID string
	Sequence  uint32
	Payload   []byte
}

func (f binaryFrame) marshalBinary() ([]byte, error) {
	if f.Type != requestBodyChunk && f.Type != responseBodyChunk {
		return nil, fmt.Errorf("unsupported binary frame type %d", f.Type)
	}
	if len(f.RequestID) == 0 || len(f.RequestID) > 1<<16-1 {
		return nil, fmt.Errorf("invalid request ID length %d", len(f.RequestID))
	}
	if len(f.Payload) > maxBinaryFramePayload {
		return nil, fmt.Errorf("binary frame payload exceeds %d bytes", maxBinaryFramePayload)
	}
	data := make([]byte, binaryFrameHeader+len(f.RequestID)+len(f.Payload))
	data[0], data[1] = binaryFrameVersion, f.Type
	binary.BigEndian.PutUint16(data[2:4], uint16(len(f.RequestID)))
	binary.BigEndian.PutUint32(data[4:8], f.Sequence)
	binary.BigEndian.PutUint32(data[8:12], uint32(len(f.Payload)))
	copy(data[12:], f.RequestID)
	copy(data[12+len(f.RequestID):], f.Payload)
	return data, nil
}

func parseBinaryFrame(data []byte) (binaryFrame, error) {
	if len(data) < binaryFrameHeader || data[0] != binaryFrameVersion {
		return binaryFrame{}, fmt.Errorf("invalid binary frame")
	}
	idLength, payloadLength := int(binary.BigEndian.Uint16(data[2:4])), int(binary.BigEndian.Uint32(data[8:12]))
	if payloadLength > maxBinaryFramePayload {
		return binaryFrame{}, fmt.Errorf("binary frame payload exceeds %d bytes", maxBinaryFramePayload)
	}
	if (data[1] != requestBodyChunk && data[1] != responseBodyChunk) || idLength == 0 || len(data) != binaryFrameHeader+idLength+payloadLength {
		return binaryFrame{}, fmt.Errorf("invalid binary frame length")
	}
	return binaryFrame{Type: data[1], RequestID: string(data[12 : 12+idLength]), Sequence: binary.BigEndian.Uint32(data[4:8]), Payload: append([]byte(nil), data[12+idLength:]...)}, nil
}
