package config

import "testing"

func TestHTTPAddrUsesHTTPAddr(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":9000")
	t.Setenv("PORT", "10000")

	if got := Load().HTTPAddr; got != ":9000" {
		t.Fatalf("HTTPAddr = %q, want :9000", got)
	}
}

func TestHTTPAddrNormalizesPort(t *testing.T) {
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("PORT", "10000")

	if got := Load().HTTPAddr; got != ":10000" {
		t.Fatalf("HTTPAddr = %q, want :10000", got)
	}
}
