package srun

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// SelfServiceClient handles communication with the Srun self-service portal.
type SelfServiceClient struct {
	BaseURL string
	log     logger
	Bind    BindOptions

	httpClient *http.Client
}

// NewSelfServiceClient creates a new self-service client.
func NewSelfServiceClient() *SelfServiceClient {
	jar, err := cookiejar.New(nil)
	if err != nil {
		jar = nil
	}
	httpClient, err := newSelfServiceHTTPClient(jar, BindOptions{})
	if err != nil {
		httpClient = &http.Client{Timeout: SelfServiceTimeout, Jar: jar}
	}
	return &SelfServiceClient{
		BaseURL:    SelfServiceDomain,
		log:        nopLogger{},
		httpClient: httpClient,
	}
}

// SetLogger sets the diagnostic logger.
func (s *SelfServiceClient) SetLogger(l logger) {
	if l == nil {
		s.log = nopLogger{}
		return
	}
	s.log = l
}

// SetVerbose enables verbose logging to stderr.
func (s *SelfServiceClient) SetVerbose(verbose bool) {
	s.log = newVerboseLogger(verbose, "SelfService")
}

// SetBind configures the outbound source IP and/or interface used for
// self-service requests. On Linux, Bind.Interface uses SO_BINDTODEVICE.
func (s *SelfServiceClient) SetBind(opts BindOptions) error {
	opts = opts.normalized()
	if err := bindClientTransport(s.httpClient, opts); err != nil {
		return err
	}
	s.Bind = opts
	return nil
}

func (s *SelfServiceClient) probeAndSetBaseURL() {
	if s.BaseURL != "" && s.BaseURL != SelfServiceDomain {
		return
	}
	s.BaseURL = resolveBaseURL(SelfServiceDomain, SelfServiceFallback, "/site/sso", "https")
}

// SSOLogin performs SSO login to the self-service portal.
func (s *SelfServiceClient) SSOLogin(username string) error {
	s.probeAndSetBaseURL()

	token := base64.StdEncoding.EncodeToString([]byte("zh-CN:" + username))
	ssoURL := s.BaseURL + "/site/sso?data=" + token

	resp, err := doRequest(s.httpClient, context.Background(), http.MethodGet, ssoURL, nil, nil, s.log)
	if err != nil {
		return wrapSelfServiceErr(err, "SSO")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("%w: HTTP %d", ErrSSORedirectedToLogin, resp.StatusCode)
	}

	if resp.Request != nil && strings.Contains(resp.Request.URL.Path, "/login") {
		return fmt.Errorf("%w: redirected to %s", ErrSSORedirectedToLogin, resp.Request.URL.Path)
	}

	s.log.Debugf("SSO login successful for user %s", username)
	return nil
}

// sessionInfo holds parsed session data from the self-service home page.
type sessionInfo struct {
	ID  string
	MAC string
}

// csrfInfo holds parsed CSRF field name and token from home page HTML.
type csrfInfo struct {
	FieldName string
	Token     string
}

var (
	// Yii2 standard: <meta name="csrf-param" content="_csrf-8800"> + csrf-token
	csrfTokenMetaRe  = regexp.MustCompile(`(?i)<meta[^>]*\bname\s*=\s*["']csrf-token["'][^>]*\bcontent\s*=\s*["']([^"']+)["']`)
	csrfTokenMetaRe2 = regexp.MustCompile(`(?i)<meta[^>]*\bcontent\s*=\s*["']([^"']+)["'][^>]*\bname\s*=\s*["']csrf-token["']`)
	csrfParamMetaRe  = regexp.MustCompile(`(?i)<meta[^>]*\bname\s*=\s*["']csrf-param["'][^>]*\bcontent\s*=\s*["']([^"']+)["']`)
	csrfParamMetaRe2 = regexp.MustCompile(`(?i)<meta[^>]*\bcontent\s*=\s*["']([^"']+)["'][^>]*\bname\s*=\s*["']csrf-param["']`)
	csrfHiddenRe     = regexp.MustCompile(`(?i)\bname\s*=\s*["'](_csrf-[^"']+)["'][^>]*\bvalue\s*=\s*["']([^"']+)["']`)
	csrfHiddenRe2    = regexp.MustCompile(`(?i)\bvalue\s*=\s*["']([^"']+)["'][^>]*\bname\s*=\s*["'](_csrf-[^"']+)["']`)
	csrfNameOnlyRe   = regexp.MustCompile(`(?i)\bname\s*=\s*["'](_csrf-[^"']+)["']`)
	deleteLinkRe     = regexp.MustCompile(`/home/delete\?id=(\d+)&(?:amp;)?user_mac=([^"&]+)`)
)

func parseCSRF(html string) (*csrfInfo, error) {
	token := firstSubmatch(html, csrfTokenMetaRe, csrfTokenMetaRe2)
	if token == "" {
		return nil, fmt.Errorf("%w: meta csrf-token missing", ErrCSRFParseFailed)
	}

	csrf := &csrfInfo{Token: token}

	// Yii2: field name in csrf-param meta (common on NWAFU self-service).
	if param := firstSubmatch(html, csrfParamMetaRe, csrfParamMetaRe2); param != "" {
		csrf.FieldName = param
		return csrf, nil
	}

	if m := csrfHiddenRe.FindStringSubmatch(html); len(m) >= 3 {
		csrf.FieldName = m[1]
		if m[2] != "" {
			csrf.Token = m[2]
		}
		return csrf, nil
	}
	if m := csrfHiddenRe2.FindStringSubmatch(html); len(m) >= 3 {
		csrf.FieldName = m[2]
		if m[1] != "" {
			csrf.Token = m[1]
		}
		return csrf, nil
	}
	if m := csrfNameOnlyRe.FindStringSubmatch(html); len(m) >= 2 {
		csrf.FieldName = m[1]
		return csrf, nil
	}

	return nil, fmt.Errorf("%w: csrf-param meta and _csrf field not found", ErrCSRFParseFailed)
}

func firstSubmatch(html string, res ...*regexp.Regexp) string {
	for _, re := range res {
		if m := re.FindStringSubmatch(html); len(m) >= 2 {
			return m[1]
		}
	}
	return ""
}

// parseHomePage extracts CSRF info and online sessions from self-service HTML.
func parseHomePage(html string) (*csrfInfo, []sessionInfo, error) {
	csrf, err := parseCSRF(html)
	if err != nil {
		return nil, nil, err
	}

	matches := deleteLinkRe.FindAllStringSubmatch(html, -1)
	var sessions []sessionInfo
	for _, m := range matches {
		mac, _ := url.QueryUnescape(m[2])
		sessions = append(sessions, sessionInfo{ID: m[1], MAC: mac})
	}
	return csrf, sessions, nil
}

// GetSessions fetches the home page and parses CSRF token + session list.
func (s *SelfServiceClient) GetSessions() (*csrfInfo, []sessionInfo, error) {
	resp, err := doRequest(s.httpClient, context.Background(), http.MethodGet, s.BaseURL+"/home", nil, nil, s.log)
	if err != nil {
		return nil, nil, wrapSelfServiceErr(err, "fetch home page")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read home page: %w", err)
	}

	csrf, sessions, err := parseHomePage(string(body))
	if err != nil {
		return nil, nil, fmt.Errorf("%w", err)
	}

	s.log.Debugf("found %d session(s)", len(sessions))
	return csrf, sessions, nil
}

// KickSession sends POST /home/delete to kick a single session.
//
// Network-layer errors (timeout, connection reset) are retried up to
// KickRetry times because TUN-mode VPN / fake-IP proxies frequently drop the
// first request before letting subsequent ones through. HTTP-status errors
// are NOT retried to avoid double-kicks.
func (s *SelfServiceClient) KickSession(id, mac string, csrf *csrfInfo) error {
	if csrf == nil || csrf.FieldName == "" {
		return errors.New("CSRF info required")
	}

	deleteURL := fmt.Sprintf("%s/home/delete?id=%s&user_mac=%s", s.BaseURL, id, url.QueryEscape(mac))

	formData := url.Values{}
	formData.Set(csrf.FieldName, csrf.Token)
	formData.Set("id", id)
	formData.Set("user_mac", mac)
	body := formData.Encode()

	extraHeaders := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
		"Referer":      s.BaseURL + "/home",
	}

	var lastErr error
	for attempt := 0; attempt <= KickRetry; attempt++ {
		resp, err := doRequest(s.httpClient, context.Background(), http.MethodPost, deleteURL, extraHeaders, strings.NewReader(body), s.log)
		if err != nil {
			lastErr = fmt.Errorf("kick request failed: %w", err)
			if attempt < KickRetry {
				s.log.Debugf("kick %s attempt %d failed: %v; retrying", id, attempt+1, err)
				sleep(KickRetryGap)
				continue
			}
			return lastErr
		}
		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("kick response read failed: %w", readErr)
			if attempt < KickRetry {
				sleep(KickRetryGap)
				continue
			}
			return lastErr
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("%w: HTTP %d", ErrKickFailed, resp.StatusCode)
		}
		if kickResponseIndicatesError(string(respBody)) {
			return fmt.Errorf("%w: server rejected kick", ErrKickFailed)
		}
		s.log.Debugf("kicked session %s", id)
		return nil
	}
	return lastErr
}

// RandomFakeMAC generates a realistic locally administered unicast MAC address.
// This avoids using fixed obvious patterns like "02:00:00:00:00:00" which can be
// blacklisted or rejected by the RADIUS/self-service portal.
func RandomFakeMAC() string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		buf = []byte{0x02, 0x54, 0xab, 0x12, 0x34, 0x56}
	}
	buf[0] = (buf[0] | 0x02) & 0xfe // Locally administered, unicast
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", buf[0], buf[1], buf[2], buf[3], buf[4], buf[5])
}

// groupSessionsByMAC groups and sorts online sessions by MAC address.
func groupSessionsByMAC(sessions []sessionInfo) map[string][]sessionInfo {
	groups := make(map[string][]sessionInfo)
	for _, sess := range sessions {
		mac := NormalizeMAC(sess.MAC)
		groups[mac] = append(groups[mac], sess)
	}
	for mac, group := range groups {
		sort.Slice(group, func(i, j int) bool {
			idI, errI := strconv.ParseInt(group[i].ID, 10, 64)
			idJ, errJ := strconv.ParseInt(group[j].ID, 10, 64)
			if errI == nil && errJ == nil {
				return idI < idJ
			}
			return group[i].ID < group[j].ID
		})
		groups[mac] = group
	}
	return groups
}

// KickAllWithFakeMAC kicks sessions using a randomly generated fake MAC and verifies
// that the targeted sessions are actually removed from the self-service online list.
// If myMAC is non-empty, only sessions matching that MAC are targeted.
// It retries up to maxRetries times if a session remains online.
func (s *SelfServiceClient) KickAllWithFakeMAC(myMAC string, maxRetries int) (int, error) {
	if maxRetries <= 0 {
		maxRetries = DefaultBypassRetries
	}

	csrf, sessions, err := s.GetSessions()
	if err != nil {
		return 0, err
	}
	if len(sessions) == 0 {
		return 0, ErrNoSessionsToKick
	}

	s.log.Infof("Found %d online session(s) before kick:", len(sessions))
	for _, sess := range sessions {
		s.log.Infof("  - id=%s mac=%s", sess.ID, sess.MAC)
	}

	filterMAC := NormalizeMAC(myMAC)
	groups := groupSessionsByMAC(sessions)

	var targets []sessionInfo
	if filterMAC != "" {
		// No -a specified: kick only the middle session of myMAC's group
		group, exists := groups[filterMAC]
		if !exists || len(group) == 0 {
			return 0, fmt.Errorf("%w: filter MAC %s", ErrNoMatchingSession, myMAC)
		}
		if len(group) <= 1 {
			s.log.Infof("Device %s already has %d session(s) (bypass active); skipping kick", myMAC, len(group))
			return 0, nil
		}
		midIdx := len(group) / 2
		targets = []sessionInfo{group[midIdx]}
		s.log.Infof("Bypass mode (no -a): device %s has %d session(s), selecting middle one (ID: %s) to kick", myMAC, len(group), targets[0].ID)
	} else {
		// -a specified: kick the middle session of ALL MAC groups
		for mac, group := range groups {
			if len(group) <= 1 {
				s.log.Infof("Device %s has %d session(s) (already bypassed); skipping", mac, len(group))
				continue
			}
			midIdx := len(group) / 2
			targets = append(targets, group[midIdx])
			s.log.Infof("Bypass mode (-a): device %s has %d session(s), selecting middle one (ID: %s) to kick", mac, len(group), group[midIdx].ID)
		}
		s.log.Infof("Bypass mode (-a): selected %d middle session(s) from %d device(s) to kick", len(targets), len(groups))
	}

	if len(targets) == 0 {
		return 0, nil
	}

	kicked := 0
	for _, target := range targets {
		sessionKicked := false
		var lastKickErr error

		for attempt := 1; attempt <= maxRetries; attempt++ {
			fakeMAC := RandomFakeMAC()
			s.log.Infof("Kick attempt %d/%d for session %s (MAC: %s) using random fake MAC %s", attempt, maxRetries, target.ID, target.MAC, fakeMAC)

			kickErr := s.KickSession(target.ID, fakeMAC, csrf)
			if kickErr != nil {
				lastKickErr = kickErr
				s.log.Warnf("Kick request attempt %d/%d for session %s failed: %v", attempt, maxRetries, target.ID, kickErr)
			}

			// Short pause to allow RADIUS/portal to process deletion
			sleep(BypassVerifyDelay)

			// Verify if session actually disappeared
			newCsrf, currentSessions, fetchErr := s.GetSessions()
			if fetchErr != nil {
				s.log.Warnf("Failed to fetch sessions for verification (attempt %d/%d): %v", attempt, maxRetries, fetchErr)
				continue
			}
			csrf = newCsrf

			stillOnline := false
			for _, cs := range currentSessions {
				if cs.ID == target.ID {
					stillOnline = true
					break
				}
			}

			if !stillOnline {
				s.log.Infof("Verified: session %s (MAC: %s) was successfully removed on attempt %d", target.ID, target.MAC, attempt)
				sessionKicked = true
				kicked++
				break
			}

			s.log.Warnf("Verification failed: session %s is STILL online in self-service after attempt %d/%d", target.ID, attempt, maxRetries)
			if attempt < maxRetries {
				sleep(KickRetryGap)
			}
		}

		if !sessionKicked {
			if lastKickErr != nil {
				return kicked, fmt.Errorf("bypass verification failed for session %s (MAC %s) after %d attempts: %w", target.ID, target.MAC, maxRetries, lastKickErr)
			}
			return kicked, fmt.Errorf("bypass verification failed for session %s (MAC %s): session remained online in self-service after %d attempts", target.ID, target.MAC, maxRetries)
		}
	}

	return kicked, nil
}

// KickAllByMAC kicks sessions using their real MAC (for actual logout).
// myMAC must be non-empty; use KickAllWithFakeMAC with macFilter "" to kick all.
func (s *SelfServiceClient) KickAllByMAC(myMAC string) (int, error) {
	if myMAC == "" {
		return 0, fmt.Errorf("%w: KickAllByMAC requires device MAC", ErrMACUndetected)
	}
	csrf, sessions, err := s.GetSessions()
	if err != nil {
		return 0, err
	}

	filter := NormalizeMAC(myMAC)
	kicked := 0
	for _, sess := range sessions {
		if NormalizeMAC(sess.MAC) != filter {
			continue
		}
		if err := s.KickSession(sess.ID, sess.MAC, csrf); err != nil {
			s.log.Debugf("failed to kick session %s: %v", sess.ID, err)
			continue
		}
		kicked++
	}
	return kicked, nil
}

// RunBypass executes SSO + fake-MAC kick + optional session check.
//
// macFilter == "" kicks every session under the account (can be more reliable,
// but also clears any other devices on the same account). A non-empty
// macFilter only kicks sessions matching that MAC.
func RunBypass(username string, macFilter string, verbose bool, checkAfter bool) (int, []sessionInfo, error) {
	return RunBypassWithBind(username, macFilter, verbose, checkAfter, BindOptions{}, DefaultBypassRetries)
}

// RunBypassWithBind executes SSO + fake-MAC kick + verification
// while forcing the self-service traffic onto a specific local IP/interface.
func RunBypassWithBind(username string, macFilter string, verbose bool, checkAfter bool, bind BindOptions, maxRetries int) (int, []sessionInfo, error) {
	ss := NewSelfServiceClient()
	ss.SetVerbose(verbose)
	if err := ss.SetBind(bind); err != nil {
		return 0, nil, err
	}

	if err := ss.SSOLogin(username); err != nil {
		return 0, nil, fmt.Errorf("bypass SSO: %w", err)
	}

	kicked, err := ss.KickAllWithFakeMAC(macFilter, maxRetries)
	if err != nil {
		return 0, nil, fmt.Errorf("bypass kick: %w", err)
	}

	if !checkAfter {
		return kicked, nil, nil
	}

	sleep(BypassCheckDelay)
	if err := ss.SSOLogin(username); err != nil {
		ss.log.Warnf("post-kick session check skipped (SSO): %v", err)
		return kicked, nil, nil
	}
	_, sessions, err := ss.GetSessions()
	if err != nil {
		ss.log.Warnf("post-kick session check skipped: %v", err)
		return kicked, nil, nil
	}
	return kicked, sessions, nil
}

func kickResponseIndicatesError(body string) bool {
	b := strings.ToLower(body)
	return strings.Contains(b, "alert-danger") || strings.Contains(b, "error-summary")
}
