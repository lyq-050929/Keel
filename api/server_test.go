package api

import "testing"

func TestGenerateSessionIDLooksRandom(t *testing.T) {
	a := generateSessionID()
	b := generateSessionID()
	if a == b {
		t.Fatalf("expected different session ids, got %s and %s", a, b)
	}
	if len(a) < 8 || len(b) < 8 {
		t.Fatalf("expected non-trivial session ids, got %q and %q", a, b)
	}
}
