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
	jar, _ := cookiejar.New(nil)
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
	csrfMetaRe   = regexp.MustCompile(`<meta name="csrf-token" content="([^"]+)"`)
	csrfInputRe  = regexp.MustCompile(`name="(_csrf-[^"]+)"\s+value="([^"]+)"`)
	deleteLinkRe = regexp.MustCompile(`/home/delete\?id=(\d+)&(?:amp;)?user_mac=([^"&]+)`)
)

// ParseHomePage extracts CSRF info and online sessions from self-service HTML.
func ParseHomePage(html string) (*CSRFInfo, []SessionInfo, error) {
	metaMatch := csrfMetaRe.FindStringSubmatch(html)
	if len(metaMatch) < 2 {
		return nil, nil, fmt.Errorf("%w: meta csrf-token missing", ErrCSRFParseFailed)
	}

	csrf := &CSRFInfo{Token: metaMatch[1]}
	if inputMatch := csrfInputRe.FindStringSubmatch(html); len(inputMatch) >= 3 {
		csrf.FieldName = inputMatch[1]
		// Prefer hidden input value when present (should match meta token).
		if inputMatch[2] != "" {
			csrf.Token = inputMatch[2]
		}
	} else {
		// Fallback: try reverse attribute order
		altRe := regexp.MustCompile(`value="([^"]+)"\s+name="(_csrf-[^"]+)"`)
		if alt := altRe.FindStringSubmatch(html); len(alt) >= 3 {
			csrf.Token = alt[1]
			csrf.FieldName = alt[2]
		}
	}
	if csrf.FieldName == "" {
		return nil, nil, fmt.Errorf("%w: hidden _csrf field missing", ErrCSRFParseFailed)
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
func (s *SelfServiceClient) KickSession(id, mac string, csrf *CSRFInfo) error {
	if csrf == nil || csrf.FieldName == "" {
		return errors.New("CSRF info required")
	}

	deleteURL := fmt.Sprintf("%s/home/delete?id=%s&user_mac=%s", s.BaseURL, id, url.QueryEscape(mac))

	formData := url.Values{}
	formData.Set(csrf.FieldName, csrf.Token)
	formData.Set("id", id)
	formData.Set("user_mac", mac)

	req, err := http.NewRequest(http.MethodPost, deleteURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", DefaultUserAgent)
	req.Header.Set("Referer", s.BaseURL+"/home")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("kick request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: HTTP %d", ErrKickFailed, resp.StatusCode)
	}

	s.log.Debugf("kicked session %s", id)
	return nil
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
func (s *SelfServiceClient) KickAllWithFakeMAC(myMAC string) (int, error) {
	csrf, sessions, err := s.GetSessions()
	if err != nil {
		return 0, err
	}

	kicked := 0
	for _, sess := range sessions {
		if myMAC != "" && sess.MAC != myMAC {
			continue
		}
		fake, err := randomMAC()
		if err != nil {
			return kicked, err
		}
		if err := s.KickSession(sess.ID, fake, csrf); err != nil {
			s.log.Warnf("failed to kick session %s: %v", sess.ID, err)
			continue
		}
		kicked++
	}
	if kicked == 0 {
		if len(sessions) == 0 {
			return 0, ErrNoSessionsToKick
		}
		if myMAC != "" {
			return 0, fmt.Errorf("%w: filter MAC %s", ErrNoMatchingSession, myMAC)
		}
	}
	return kicked, nil
}

// KickAllByMAC kicks sessions using their real MAC (for actual logout).
func (s *SelfServiceClient) KickAllByMAC(myMAC string) (int, error) {
	csrf, sessions, err := s.GetSessions()
	if err != nil {
		return 0, err
	}

	kicked := 0
	for _, sess := range sessions {
		if myMAC != "" && sess.MAC != myMAC {
			continue
		}
		if err := s.KickSession(sess.ID, sess.MAC, csrf); err != nil {
			s.log.Warnf("failed to kick session %s: %v", sess.ID, err)
			continue
		}
		kicked++
	}
	return kicked, nil
}

// RunBypass executes SSO + fake-MAC kick + optional session check.
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
		return kicked, nil, fmt.Errorf("post-kick SSO failed: %w", err)
	}
	_, sessions, err := ss.GetSessions()
	if err != nil {
		return kicked, nil, fmt.Errorf("post-kick session check failed: %w", err)
	}
	return kicked, sessions, nil
}
