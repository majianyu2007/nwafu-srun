package srun

import "testing"

func TestNormalizeMAC(t *testing.T) {
	a := NormalizeMAC("8C-85-90-05-93-45")
	b := NormalizeMAC("8c:85:90:05:93:45")
	if a != b {
		t.Fatalf("NormalizeMAC mismatch: %q vs %q", a, b)
	}
}

func TestMACEqual(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"8C-85-90-05-93-45", "8c:85:90:05:93:45", true},
		{"8C-85-90-05-93-45", "8C-85-90-05-93-46", false},
		{" 8c:85:90:05:93:45 ", "8c:85:90:05:93:45", true},
	}
	for _, tt := range tests {
		if got := MACEqual(tt.a, tt.b); got != tt.want {
			t.Errorf("MACEqual(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

