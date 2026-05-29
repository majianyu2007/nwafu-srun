package srun

import (
	"errors"
	"fmt"
	"net"
	"strings"
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
	// ErrStayOnline signals the user chose to keep the existing online session.
	ErrStayOnline = errors.New("stay online")
)

// Hint returns a user-facing remediation suggestion for known errors.
func Hint(err error) string {
	if err == nil {
		return ""
	}
	if h := proxyHint(err); h != "" {
		return h
	}
	switch {
	case errors.Is(err, ErrNotOnline):
		return "Authenticate first (menu 1 or run with -u/-p). Check that you are on campus network."
	case errors.Is(err, ErrPortalUnreachable):
		return "Cannot reach the portal. Check Wi-Fi/cable, DNS, or try again on campus network."
	case errors.Is(err, ErrSelfServiceUnreachable):
		return "Cannot reach the self-service portal. If you are using a TUN-mode VPN/proxy (Clash, Mihomo, v2rayN, etc.), add 'service.nwafu.edu.cn' and 'portal.nwafu.edu.cn' to its direct/bypass rules."
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

// proxyHint inspects the error chain for symptoms of a TUN-mode VPN /
// fake-IP proxy intercepting campus traffic and returns a targeted hint.
// Common symptoms: "context deadline exceeded" on POST while GETs work, or
// connection forcibly closed by 198.18.x.x (Clash/Mihomo fake-IP range).
func proxyHint(err error) string {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "198.18."),
		strings.Contains(msg, "forcibly closed"),
		strings.Contains(msg, "connection reset"),
		strings.Contains(msg, "deadline exceeded") && (strings.Contains(msg, "service.nwafu") || strings.Contains(msg, "portal.nwafu") || strings.Contains(msg, "kick")):
		return "Network request to a campus host timed out or was reset. " +
			"If you are running a TUN-mode VPN / proxy software (Clash, Mihomo, v2rayN, Surge, ...), " +
			"add 'service.nwafu.edu.cn', 'portal.nwafu.edu.cn' and 172.26.0.0/16 to its direct/bypass list, " +
			"or temporarily disable the proxy and retry."
	}
	return ""
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
