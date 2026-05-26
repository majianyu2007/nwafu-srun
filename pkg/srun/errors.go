package srun

import (
	"errors"
	"fmt"
	"net"
)

var (
	ErrNotOnline              = errors.New("not online")
	ErrPortalUnreachable      = errors.New("portal unreachable")
	ErrSelfServiceUnreachable = errors.New("self-service unreachable")
	ErrSSORedirectedToLogin   = errors.New("SSO redirected to login")
	ErrCSRFParseFailed        = errors.New("CSRF parse failed")
	ErrNoMatchingSession      = errors.New("no session matched the given MAC")
	ErrMACUndetected          = errors.New("local MAC undetected")
	ErrAuthFailed             = errors.New("auth failed")
	ErrKickFailed             = errors.New("kick session failed")
	ErrNoSessionsToKick       = errors.New("no online sessions to kick")
)

// Hint returns a user-facing remediation suggestion for known errors.
func Hint(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, ErrNotOnline):
		return "Authenticate first (menu 1 or run with -u/-p). Check that you are on campus network."
	case errors.Is(err, ErrPortalUnreachable):
		return "Cannot reach the portal. Check Wi-Fi/cable, DNS, or try again on campus network."
	case errors.Is(err, ErrSelfServiceUnreachable):
		return "Cannot reach the self-service portal. Check network; portal login may still work."
	case errors.Is(err, ErrSSORedirectedToLogin):
		return "You don't appear to be authenticated on the portal. Run Login (menu 1) first, then retry bypass."
	case errors.Is(err, ErrCSRFParseFailed):
		return "Self-service page layout may have changed, or SSO did not complete. Login on portal, then retry."
	case errors.Is(err, ErrNoMatchingSession):
		return "No session matches your device MAC. Confirm you are online, or use -a/--all to kick all devices on the account."
	case errors.Is(err, ErrMACUndetected):
		return "Could not read your device MAC from portal status. Login first, or use -a/--all if you intend to kick every session."
	case errors.Is(err, ErrAuthFailed):
		return "Check username/password and ac_id (--acid). Use -v for response details on stderr."
	case errors.Is(err, ErrKickFailed):
		return "Kick request failed. Retry bypass; use -v to see HTTP details."
	case errors.Is(err, ErrNoSessionsToKick):
		return "No sessions listed on self-service. You may already be bypassed or not online."
	default:
		return ""
	}
}

// FormatError returns err string plus optional Hint line.
func FormatError(err error) string {
	if err == nil {
		return ""
	}
	if h := Hint(err); h != "" {
		return fmt.Sprintf("%v\nHint: %s", err, h)
	}
	return err.Error()
}

func wrapPortalErr(err error, msg string) error {
	if err == nil {
		return nil
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return fmt.Errorf("%s: %w: %v", msg, ErrPortalUnreachable, err)
	}
	if errors.Is(err, ErrNotOnline) || errors.Is(err, ErrAuthFailed) {
		return err
	}
	return fmt.Errorf("%s: %w", msg, err)
}

func wrapSelfServiceErr(err error, msg string) error {
	if err == nil {
		return nil
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return fmt.Errorf("%s: %w: %v", msg, ErrSelfServiceUnreachable, err)
	}
	return fmt.Errorf("%s: %w", msg, err)
}
