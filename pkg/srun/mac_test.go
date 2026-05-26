package srun

import "testing"

func TestNormalizeMAC(t *testing.T) {
	a := normalizeMAC("8C-85-90-05-93-45")
	b := normalizeMAC("8c:85:90:05:93:45")
	if a != b {
		t.Fatalf("normalizeMAC mismatch: %q vs %q", a, b)
	}
}
