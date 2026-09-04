package tunnels

import (
	"testing"
)

func TestRegistryZeroValueMetrics(t *testing.T) {
	m := TrafficMetrics{}
	if m.RequestsTotal != 0 || m.RequestBytes != 0 || m.ResponseBytes != 0 {
		t.Fatal("zero TrafficMetrics should have all fields zero")
	}
}

func TestRegistryDrainEmptyReturnsEmpty(t *testing.T) {
	r := NewRegistry()
	deltas := r.DrainUserDeltas()
	if len(deltas) != 0 {
		t.Fatalf("drain on empty registry should return empty map, got %d entries", len(deltas))
	}
}

func TestRegistryDrainTwiceGetsEmpty(t *testing.T) {
	r := NewRegistry()
	_ = r.DrainUserDeltas()
	deltas := r.DrainUserDeltas()
	if len(deltas) != 0 {
		t.Fatalf("second drain should be empty, got %d entries", len(deltas))
	}
}

func TestActiveTrafficDelta(t *testing.T) {
	delta := TrafficMetrics{
		RequestsTotal:  10,
		RequestBytes:   1024,
		ResponseBytes:  2048,
	}
	total := delta.RequestsTotal + delta.RequestBytes + delta.ResponseBytes
	if total != 3082 {
		t.Fatalf("delta sum should be 3082, got %d", total)
	}
}
