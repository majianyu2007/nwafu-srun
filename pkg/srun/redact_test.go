package srun

import (
	"strings"
	"testing"
)

func TestRedactSensitive(t *testing.T) {
	in := `GET /login?password=secret&info=SRBX1data&chksum=abc123`
	out := redactSensitive(in)
	if out == in {
		t.Fatal("expected redaction")
	}
	for _, leak := range []string{"secret", "SRBX1", "abc123"} {
		if strings.Contains(out, leak) {
			t.Fatalf("leaked %q in %q", leak, out)
		}
	}
}
