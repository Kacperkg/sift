package securestore

import "testing"

func TestRandomToken(t *testing.T) {
	a, err := RandomToken(32)
	if err != nil || len(a) < 40 {
		t.Fatalf("bad token: %q %v", a, err)
	}
}
