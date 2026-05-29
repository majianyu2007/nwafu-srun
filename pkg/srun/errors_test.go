package srun

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestErrorsIs(t *testing.T) {
	cases := []error{
		ErrNotOnline,
		ErrPortalUnreachable,
		ErrSelfServiceUnreachable,
		ErrSSORedirectedToLogin,
		ErrCSRFParseFailed,
		ErrNoMatchingSession,
		ErrMACUndetected,
		ErrAuthFailed,
		ErrKickFailed,
		ErrNoSessionsToKick,
		ErrStayOnline,
	}
	for _, want := range cases {
		got := fmt.Errorf("outer: %w", want)
		if !errors.Is(got, want) {
			t.Fatalf("errors.Is failed for %v", want)
		}
	}
}

func TestHintKnown(t *testing.T) {
	if Hint(ErrNotOnline) == "" {
		t.Fatal("expected hint for ErrNotOnline")
	}
}

func TestHintUnknown(t *testing.T) {
	if Hint(errors.New("random")) != "" {
		t.Fatal("expected empty hint for unknown error")
	}
}

func TestHintTUNProxySymptoms(t *testing.T) {
	cases := []error{
		errors.New(`Post "https://service.nwafu.edu.cn/home/index": read tcp 198.18.0.1:9718->198.18.0.11:443: wsarecv: An existing connection was forcibly closed by the remote host.`),
		errors.New(`kick request failed: Post "https://service.nwafu.edu.cn/home/delete": context deadline exceeded`),
		errors.New(`dial tcp 198.18.0.11:443: connectex: A connection attempt failed`),
	}
	for _, e := range cases {
		h := Hint(e)
		if h == "" {
			t.Fatalf("expected proxy hint for %q", e)
		}
		if !strings.Contains(h, "TUN") {
			t.Fatalf("hint should mention TUN proxy, got: %s", h)
		}
	}
}
