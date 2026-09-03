package agenttokens

import "testing"

func TestGenerateProducesOpaquePrefixedTokens(t *testing.T) {
	t.Parallel()
	first, err := generate()
	if err != nil {
		t.Fatalf("generate first token: %v", err)
	}
	second, err := generate()
	if err != nil {
		t.Fatalf("generate second token: %v", err)
	}
	if first == second || len(first) < 40 {
		t.Fatal("tokens must be distinct, high-entropy values")
	}
	if visiblePrefix(first) == first {
		t.Fatal("visible prefix must not expose the entire token")
	}
	if hash(first) == first {
		t.Fatal("token hash must not equal token plaintext")
	}
}

func TestVisiblePrefixKeepsShortValuesAndTruncatesLongTokens(t *testing.T) {
	t.Parallel()
	if got := visiblePrefix("short"); got != "short" {
		t.Fatalf("short prefix=%q", got)
	}
	if got := visiblePrefix("123456789012345"); got != "123456789012" {
		t.Fatalf("long prefix=%q", got)
	}
}
