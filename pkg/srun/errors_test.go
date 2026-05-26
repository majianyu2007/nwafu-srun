package srun

import (
	"errors"
	"fmt"
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

func TestFormatError(t *testing.T) {
	s := FormatError(ErrSSORedirectedToLogin)
	if s == "" {
		t.Fatal("empty format")
	}
	if Hint(ErrSSORedirectedToLogin) != "" && !errors.Is(fmt.Errorf("%s", s), ErrSSORedirectedToLogin) {
		// FormatError should mention hint when available
		if len(s) < 20 {
			t.Fatalf("format too short: %q", s)
		}
	}
}
