package traffic

import (
	"testing"
)

func TestMetricsZeroValue(t *testing.T) {
	m := Metrics{}
	if m.RequestsTotal != 0 || m.RequestBytes != 0 || m.ResponseBytes != 0 {
		t.Fatal("zero-value Metrics should have all fields zero")
	}
}
