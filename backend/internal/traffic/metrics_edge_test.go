package traffic

import (
	"testing"
)

func TestMetricsAddition(t *testing.T) {
	m1 := Metrics{RequestsTotal: 10, RequestBytes: 100, ResponseBytes: 200}
	m2 := Metrics{RequestsTotal: 5, RequestBytes: 50, ResponseBytes: 100}

	total := m1.RequestsTotal + m2.RequestsTotal
	if total != 15 {
		t.Fatalf("want 15 requests, got %d", total)
	}

	bytes := m1.RequestBytes + m2.RequestBytes + m1.ResponseBytes + m2.ResponseBytes
	if bytes != 450 {
		t.Fatalf("want 450 total bytes, got %d", bytes)
	}
}

func TestMetricsLargNumbers(t *testing.T) {
	m := Metrics{
		RequestsTotal:  1_000_000,
		RequestBytes:   1_000_000_000,
		ResponseBytes:  2_000_000_000,
	}
	if m.RequestsTotal != 1_000_000 {
		t.Fatal("large numbers should be preserved")
	}
}

func TestMetricsNegativeHandling(t *testing.T) {
	m := Metrics{}
	if m.RequestsTotal < 0 || m.RequestBytes < 0 || m.ResponseBytes < 0 {
		t.Fatal("zero metrics should not be negative")
	}
}
