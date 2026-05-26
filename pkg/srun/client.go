package srun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// LoginInfo holds parsed portal user status.
type LoginInfo struct {
	Username string
	Balance  string
	UsedMB   float64
	MAC      string
	IP       string
}

// Client handles communication with the Srun authentication portal.
type Client struct {
	Username string
	Password string
	IP       string
	MAC      string
	AcID     string
	BaseURL  string

	log        Logger
	httpClient *http.Client
}

// NewClient creates a new Srun Client instance.
func NewClient(username, password, acid string) *Client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		jar = nil
	}
	return &Client{
		Username:   username,
		Password:   password,
		AcID:       acid,
		BaseURL:    PortalDomain,
		log:        NopLogger{},
		httpClient: newPortalHTTPClient(jar),
	}
}

// SetLogger sets the diagnostic logger for this client.
func (c *Client) SetLogger(l Logger) {
	if l == nil {
		c.log = NopLogger{}
		return
	}
	c.log = l
}

// SetVerbose enables or disables verbose logging via stderr.
func (c *Client) SetVerbose(verbose bool) {
	c.log = NewVerboseLogger(verbose, "Portal")
}

func (c *Client) ctx() context.Context {
	return context.Background()
}

// resolveBaseURL tries three-layer DNS fallback for a portal host.
func resolveBaseURL(domainURL, fallbackIP, testPath, fallbackScheme string) string {
	probe := newProbeClient()
	testURL := domainURL + testPath
	if probeURL(probe, testURL) {
		return domainURL
	}

	host := strings.TrimPrefix(strings.TrimPrefix(domainURL, "https://"), "http://")
	for _, dns := range CampusDNSServers() {
		r := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
				d := net.Dialer{Timeout: DNSTimeout}
				return d.DialContext(ctx, "udp", dns+":53")
			},
		}
		ips, err := r.LookupHost(context.Background(), host)
		if err != nil {
			continue
		}
		for _, ip := range ips {
			ipTest := fallbackScheme + "://" + ip + testPath
			if probeURL(probe, ipTest) {
				return fallbackScheme + "://" + ip
			}
		}
	}

	return fallbackScheme + "://" + fallbackIP
}

func (c *Client) probeAndSetBaseURL() {
	if c.BaseURL != "" && c.BaseURL != PortalDomain {
		return
	}
	c.BaseURL = resolveBaseURL(PortalDomain, PortalFallback, "/srun_portal_pc?ac_id="+c.AcID+"&theme=pro", "http")
}

func (c *Client) getHostLoginPageURL() string {
	return c.BaseURL + "/srun_portal_pc?ac_id=" + c.AcID + "&theme=pro"
}

func (c *Client) getChallengeURL() string {
	return c.BaseURL + "/cgi-bin/get_challenge"
}

func (c *Client) getLogInURL() string {
	return c.BaseURL + "/cgi-bin/srun_portal"
}

func (c *Client) getLoginInfoURL() string {
	return c.BaseURL + "/cgi-bin/rad_user_info"
}

func (c *Client) portalHost() string {
	return strings.TrimPrefix(strings.TrimPrefix(c.BaseURL, "https://"), "http://")
}

// GetIP fetches the current client IP from the portal login page.
func (c *Client) GetIP() (string, error) {
	c.probeAndSetBaseURL()

	req, err := http.NewRequestWithContext(c.ctx(), http.MethodGet, c.getHostLoginPageURL(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", DefaultUserAgent)

	c.log.Debugf("GET %s", c.getHostLoginPageURL())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", wrapPortalErr(err, "get IP")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read login page: %w", err)
	}
	c.log.Debugf("login page response len=%d", len(body))

	re := regexp.MustCompile(`ip\s*:\s*"(.*?)"`)
	matches := re.FindStringSubmatch(string(body))
	if len(matches) > 1 {
		c.IP = matches[1]
		return c.IP, nil
	}
	return "", fmt.Errorf("%w: IP not found on login page", ErrPortalUnreachable)
}

// GetChallenge gets the authentication token challenge.
func (c *Client) GetChallenge() (string, error) {
	u, err := url.Parse(c.getChallengeURL())
	if err != nil {
		return "", err
	}
	q := u.Query()
	nowMs := strconv.FormatInt(time.Now().UnixNano()/1e6, 10)
	q.Set("callback", "jQuery11277455887669735664_"+nowMs)
	q.Set("username", c.Username)
	q.Set("ip", c.IP)
	q.Set("_", nowMs)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(c.ctx(), http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", DefaultUserAgent)

	c.log.Debugf("GET %s", u.String())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", wrapPortalErr(err, "get challenge")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read challenge: %w", err)
	}
	c.log.Debugf("challenge response: %s", string(body))

	re := regexp.MustCompile(`"challenge":"(.*?)"`)
	matches := re.FindStringSubmatch(string(body))
	if len(matches) > 1 {
		return matches[1], nil
	}
	return "", fmt.Errorf("%w: challenge token not in response", ErrPortalUnreachable)
}

type loginInfoPayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
	IP       string `json:"ip"`
	AcID     string `json:"acid"`
	EncVer   string `json:"enc_ver"`
}

func (c *Client) getInfoString(challenge string) (string, error) {
	payload := loginInfoPayload{
		Username: c.Username,
		Password: c.Password,
		IP:       c.IP,
		AcID:     c.AcID,
		EncVer:   "srun_bx1",
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	encoded := jsBase64(xencode(string(b), challenge))
	return "{SRBX1}" + encoded, nil
}

func (c *Client) chksumAdd(challenge, md5Info, info string) string {
	str := challenge + c.Username
	str += challenge + md5Info
	str += challenge + c.AcID
	str += challenge + c.IP
	str += challenge + "200"
	str += challenge + "1"
	str += challenge + info
	return str
}

// LogIn attempts portal login and refreshes login info on success.
func (c *Client) LogIn() (*LoginInfo, error) {
	if c.IP == "" {
		if _, err := c.GetIP(); err != nil {
			return nil, err
		}
	}

	challenge, err := c.GetChallenge()
	if err != nil {
		return nil, err
	}

	md5Info := HMACMD5Hex(c.Password, challenge)
	md5Str := "{MD5}" + md5Info
	infoStr, err := c.getInfoString(challenge)
	if err != nil {
		return nil, err
	}
	chksumStr := SHA1Hex(c.chksumAdd(challenge, md5Info, infoStr))

	u, err := url.Parse(c.getLogInURL())
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("callback", "jQuery1124064")
	q.Set("action", "login")
	q.Set("username", c.Username)
	q.Set("password", md5Str)
	q.Set("ac_id", c.AcID)
	q.Set("ip", c.IP)
	q.Set("info", infoStr)
	q.Set("chksum", chksumStr)
	q.Set("n", "200")
	q.Set("type", "1")
	q.Set("_", strconv.FormatInt(time.Now().UnixNano()/1e6, 10))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(c.ctx(), http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/javascript, application/javascript, application/ecmascript, application/x-ecmascript, */*; q=0.01")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Connection", "keep-alive")
	req.AddCookie(&http.Cookie{Name: "lang", Value: "zh-CN"})
	req.Header.Set("Host", c.portalHost())
	req.Header.Set("Referer", c.getHostLoginPageURL())
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("User-Agent", DefaultUserAgent)

	c.log.Debugf("GET %s", redactSensitive(u.String()))
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, wrapPortalErr(err, "login")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read login response: %w", err)
	}
	c.log.Debugf("login response: %s", redactSensitive(string(body)))

	re := regexp.MustCompile(`"res":"(.*?)"`)
	matches := re.FindStringSubmatch(string(body))
	res := ""
	if len(matches) > 1 {
		res = matches[1]
	}
	errRe := regexp.MustCompile(`"error":"(.*?)"`)
	errMatches := errRe.FindStringSubmatch(string(body))
	errStr := ""
	if len(errMatches) > 1 {
		errStr = errMatches[1]
	}
	errMsgRe := regexp.MustCompile(`"error_msg":"(.*?)"`)
	errMsgMatches := errMsgRe.FindStringSubmatch(string(body))
	errMsg := ""
	if len(errMsgMatches) > 1 {
		errMsg = errMsgMatches[1]
	}
	if loginSucceeded(res, errStr, errMsg) {
		sleep(LoginSettleDelay)
		var lastErr error
		for i := 0; i < LoginInfoRetry; i++ {
			info, infoErr := c.GetLoginInfo()
			if infoErr == nil {
				return info, nil
			}
			lastErr = infoErr
			if errors.Is(infoErr, ErrNotOnline) {
				sleep(LoginInfoRetryGap)
				continue
			}
			return nil, infoErr
		}
		if errors.Is(lastErr, ErrNotOnline) {
			// Some gateways return transient "not_online_error" immediately after
			// reporting login success. Treat login as successful and return a
			// minimal info block so interactive UX remains consistent.
			info := &LoginInfo{
				Username: c.Username,
				IP:       c.IP,
				Balance:  "0.00",
				MAC:      c.MAC,
			}
			if info.MAC == "" {
				c.log.Warnf("device MAC not detected; bypass without -a/--all may fail")
			}
			return info, nil
		}
		return nil, lastErr
	}

	if errMsg == "" {
		errMsg = parsePortalError(string(body), matches)
	}
	return nil, fmt.Errorf("%w: %s", ErrAuthFailed, errMsg)
}

var (
	loginSuccessRe = regexp.MustCompile(`(?i)\b(success|welcome|ok)\b`)
	loginFailRe    = regexp.MustCompile(`(?i)\b(unsuccessful|fail|failed|invalid|incorrect|wrong|error)\b`)
)

// loginSucceeded recognises Srun login success responses.
//
// Srun deployments are inconsistent about where the success marker lives:
//   - Some return {"res":"ok"} or {"error":"ok"}.
//   - Some return {"res":"login_error","error":"login_error",
//     "error_msg":"Authentication success,Welcome!"} after a successful login.
//
// Matching uses word boundaries so "unsuccessful" is not treated as "success".
func loginSucceeded(res, errStr, errMsg string) bool {
	if res == "ok" || errStr == "ok" {
		return true
	}
	combined := res + " " + errStr + " " + errMsg
	if loginFailRe.MatchString(combined) {
		return false
	}
	return loginSuccessRe.MatchString(combined)
}

func parsePortalError(body string, resMatches []string) string {
	reErr := regexp.MustCompile(`"error_msg":"(.*?)"`)
	if m := reErr.FindStringSubmatch(body); len(m) > 1 {
		return m[1]
	}
	if len(resMatches) > 1 {
		return resMatches[1]
	}
	return "unknown error"
}

// GetLoginInfo queries current online status and populates Client fields.
func (c *Client) GetLoginInfo() (*LoginInfo, error) {
	if c.IP == "" {
		if _, err := c.GetIP(); err != nil {
			return nil, err
		}
	}

	u, err := url.Parse(c.getLoginInfoURL())
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("callback", "jQuery112402812915")
	q.Set("_", strconv.FormatInt(time.Now().Unix(), 10))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(c.ctx(), http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", DefaultUserAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Referer", c.getHostLoginPageURL())

	c.log.Debugf("GET %s", u.String())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, wrapPortalErr(err, "get status")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read status response: %w", err)
	}
	c.log.Debugf("status response: %s", string(body))

	info, err := parseLoginInfo(string(body), c.IP)
	if err != nil {
		return nil, err
	}
	if info.MAC != "" {
		c.MAC = normalizeMAC(info.MAC)
		info.MAC = c.MAC
	}
	return info, nil
}

func parseLoginInfo(strLoginInfo, ip string) (*LoginInfo, error) {
	reErr := regexp.MustCompile(`"error":"(.*?)"`)
	errMatch := reErr.FindStringSubmatch(strLoginInfo)
	if len(errMatch) < 2 || errMatch[1] != "ok" {
		errInfo := "unknown"
		if len(errMatch) > 1 {
			errInfo = errMatch[1]
		}
		return nil, fmt.Errorf("%w: %s", ErrNotOnline, errInfo)
	}

	reUser := regexp.MustCompile(`"user_name":"([^"]*)"`)
	reBalance := regexp.MustCompile(`"user_balance":(.*?),`)
	reSumBytes := regexp.MustCompile(`"sum_bytes":(\d+),`)
	reMAC := regexp.MustCompile(`"user_mac":"(([0-9A-Fa-f]{2}[:-]){5}[0-9A-Fa-f]{2})"`)

	info := &LoginInfo{IP: ip, Balance: "0.00"}
	if m := reUser.FindStringSubmatch(strLoginInfo); len(m) > 1 {
		info.Username = m[1]
	}
	if m := reBalance.FindStringSubmatch(strLoginInfo); len(m) > 1 {
		info.Balance = m[1]
	}
	if m := reSumBytes.FindStringSubmatch(strLoginInfo); len(m) > 1 {
		bytesVal, _ := strconv.ParseFloat(m[1], 64)
		info.UsedMB = bytesVal / 1_000_000.0
	}
	if m := reMAC.FindStringSubmatch(strLoginInfo); len(m) > 1 {
		info.MAC = normalizeMAC(m[1])
	}
	return info, nil
}

// FormatLoginInfo returns a human-readable status block.
func FormatLoginInfo(info *LoginInfo) string {
	return formatLoginInfo(info, "Login successfully")
}

// FormatStatusInfo returns a human-readable status block for status query.
func FormatStatusInfo(info *LoginInfo) string {
	return formatLoginInfo(info, "Current online status")
}

func formatLoginInfo(info *LoginInfo, title string) string {
	if info == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n-----------------------------------------\n")
	b.WriteString(fmt.Sprintf("%-20s-%20s\n", title, ""))
	b.WriteString(fmt.Sprintf("%-20s-%20s\n", "     User name", info.Username))
	b.WriteString(fmt.Sprintf("%-20s-%20s\n", "            IP", info.IP))
	b.WriteString(fmt.Sprintf("%-20s-%20s\n", "       Balance", info.Balance))
	b.WriteString(fmt.Sprintf("%-20s-%20.2f\n", "       Used MB", info.UsedMB))
	if info.MAC != "" {
		b.WriteString(fmt.Sprintf("%-20s-%20s\n", "           MAC", info.MAC))
	}
	b.WriteString("-----------------------------------------\n")
	return b.String()
}

// LogOut attempts portal logout, falling back to self-service kick.
func (c *Client) LogOut() error {
	return c.logOutInternal(false)
}

// QuietLogOut attempts logout without printing; errors are returned.
func (c *Client) QuietLogOut() error {
	return c.logOutInternal(true)
}

func (c *Client) logOutInternal(quiet bool) error {
	if c.IP == "" {
		if _, err := c.GetIP(); err != nil {
			return err
		}
	}

	u, err := url.Parse(c.getLogInURL())
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("callback", "jQuery11240579338170130")
	q.Set("action", "logout")
	q.Set("ac_id", c.AcID)
	q.Set("ip", c.IP)
	q.Set("username", c.Username)
	q.Set("_", strconv.FormatInt(time.Now().UnixNano()/1e6, 10))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(c.ctx(), http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", DefaultUserAgent)

	c.log.Debugf("GET %s", u.String())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return wrapPortalErr(err, "logout")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read logout response: %w", err)
	}
	c.log.Debugf("logout response: %s", string(body))

	re := regexp.MustCompile(`"res":"(.*?)"`)
	matches := re.FindStringSubmatch(string(body))
	if len(matches) > 1 && matches[1] == "ok" {
		return nil
	}

	if quiet {
		return errors.New("portal logout failed")
	}

	// Portal logout often fails; try self-service kick as fallback (own MAC only).
	if c.MAC == "" {
		if info, err := c.GetLoginInfo(); err == nil && info.MAC != "" {
			c.MAC = info.MAC
		}
	}
	if c.MAC == "" {
		return fmt.Errorf("portal logout failed and %w: cannot kick sessions without device MAC", ErrMACUndetected)
	}

	c.log.Infof("portal logout unavailable, trying self-service kick")
	selfSvc := NewSelfServiceClient()
	selfSvc.SetLogger(c.log)
	if err := selfSvc.SSOLogin(c.Username); err != nil {
		return fmt.Errorf("SSO failed: %w", err)
	}
	kicked, err := selfSvc.KickAllByMAC(c.MAC)
	if err != nil {
		return fmt.Errorf("self-service kick failed: %w", err)
	}
	if kicked == 0 {
		return errors.New("fail to logout: no sessions kicked")
	}
	return nil
}
