package tunnels

import (
	"regexp"
	"testing"
)

func TestGenerateSubdomainUsesDNSLabelFormat(t *testing.T) {
	value, err := GenerateSubdomain()
	if err != nil {
		t.Fatalf("GenerateSubdomain() error = %v", err)
	}
	if !regexp.MustCompile(`^[a-z0-9]{10}$`).MatchString(value) {
		t.Fatalf("subdomain %q is not DNS-label safe", value)
	}
}

func TestGenerateSubdomainDoesNotRepeatInSample(t *testing.T) {
	seen := make(map[string]struct{})
	for range 500 {
		value, err := GenerateSubdomain()
		if err != nil {
			t.Fatal(err)
		}
		if _, exists := seen[value]; exists {
			t.Fatalf("duplicate subdomain %q", value)
		}
		seen[value] = struct{}{}
	}
}
