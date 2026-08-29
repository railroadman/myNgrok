package tunnels

import "testing"

func TestParsePublicHost(t *testing.T) {
	tests := []struct {
		host string
		want string
		ok   bool
	}{
		{"demo123.tunnel.example.test", "demo123", true}, {"DEMO123.tunnel.example.test:8080", "demo123", true}, {"tunnel.example.test", "", false}, {"one.two.tunnel.example.test", "", false}, {"demo123.other.test", "", false}, {"bad_thing.tunnel.example.test", "", false},
	}
	for _, test := range tests {
		got, ok := ParsePublicHost(test.host, "tunnel.example.test")
		if got != test.want || ok != test.ok {
			t.Errorf("ParsePublicHost(%q) = (%q,%v), want (%q,%v)", test.host, got, ok, test.want, test.ok)
		}
	}
}
