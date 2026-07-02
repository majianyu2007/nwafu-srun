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
	Username   string
	Password   string
	IP         string
	MAC        string
	AcID       string
	BaseURL    string
	LogoutMode string
	Bind       BindOptions

	log        logger
	httpClient *http.Client
}

// NewClient creates a new Srun Client instance.
func NewClient(username, password, acid string) *Client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		jar = nil
	}
	httpClient, err := newPortalHTTPClient(jar, BindOptions{})
	if err != nil {
		httpClient = &http.Client{Timeout: PortalTimeout, Jar: jar}
	}
	return &Client{
		Username:   username,
		Password:   password,
		AcID:       acid,
		BaseURL:    PortalDomain,
		log:        nopLogger{},
		httpClient: httpClient,
	}
}

// SetLogger sets the diagnostic logger for this client.
func (c *Client) SetLogger(l logger) {
	if l == nil {
		c.log = nopLogger{}
		return
	}
	c.log = l
}

// SetVerbose enables or disables verbose logging via stderr.
func (c *Client) SetVerbose(verbose bool) {
	c.log = newVerboseLogger(verbose, "Portal")
}

// SetBind configures the outbound source IP and/or interface used for portal
// requests. On Linux, Bind.Interface uses SO_BINDTODEVICE.
func (c *Client) SetBind(opts BindOptions) error {
	opts = opts.normalized()
	if err := bindClientTransport(c.httpClient, opts); err != nil {
		return err
	}
	c.Bind = opts
	return nil
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
	for _, dns := range campusDNSServers() {
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

	body, err := c.getBytes(c.getHostLoginPageURL(), nil)
	if err != nil {
		return "", wrapPortalErr(err, "get IP")
	}
	c.log.Debugf("login page response len=%d", len(body))

	matches := reIP.FindStringSubmatch(string(body))
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

	body, err := c.getBytes(u.String(), nil)
	if err != nil {
		return "", wrapPortalErr(err, "get challenge")
	}
	c.log.Debugf("challenge response: %s", string(body))

	matches := reChallenge.FindStringSubmatch(string(body))
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

	md5Info := hmacMD5Hex(c.Password, challenge)
	md5Str := "{MD5}" + md5Info
	infoStr, err := c.getInfoString(challenge)
	if err != nil {
		return nil, err
	}
	chksumStr := sha1Hex(c.chksumAdd(challenge, md5Info, infoStr))

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

	extraHeaders := map[string]string{
		"Accept":           "text/javascript, application/javascript, application/ecmascript, application/x-ecmascript, */*; q=0.01",
		"Accept-Language":  "zh-CN,zh;q=0.9,en;q=0.8",
		"Connection":       "keep-alive",
		"Cookie":           "lang=zh-CN",
		"Host":             c.portalHost(),
		"Referer":          c.getHostLoginPageURL(),
		"X-Requested-With": "XMLHttpRequest",
	}
	body, err := c.getBytes(u.String(), extraHeaders)
	if err != nil {
		return nil, wrapPortalErr(err, "login")
	}
	c.log.Debugf("login response: %s", redactSensitive(string(body)))

	matches := reRes.FindStringSubmatch(string(body))
	res := ""
	if len(matches) > 1 {
		res = matches[1]
	}
	errMatches := reError.FindStringSubmatch(string(body))
	errStr := ""
	if len(errMatches) > 1 {
		errStr = errMatches[1]
	}
	errMsgMatches := reErrorMsg.FindStringSubmatch(string(body))
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

// Pre-compiled regexes for parsing portal responses.
var (
	reIP        = regexp.MustCompile(`ip\s*:\s*"(.*?)"`)
	reChallenge = regexp.MustCompile(`"challenge":"(.*?)"`)
	reRes       = regexp.MustCompile(`"res":"(.*?)"`)
	reError     = regexp.MustCompile(`"error":"(.*?)"`)
	reErrorMsg  = regexp.MustCompile(`"error_msg":"(.*?)"`)
	reUser      = regexp.MustCompile(`"user_name":"([^"]*)"`)
	reBalance   = regexp.MustCompile(`"user_balance":(.*?),`)
	reSumBytes  = regexp.MustCompile(`"sum_bytes":(\d+),`)
	reMAC       = regexp.MustCompile(`"user_mac":"(([0-9A-Fa-f]{2}[:-]){5}[0-9A-Fa-f]{2})"`)
)

// loginSucceeded recognizes Srun login success responses.
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
	if m := reErrorMsg.FindStringSubmatch(body); len(m) > 1 {
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

	extraHeaders := map[string]string{
		"Accept":  "*/*",
		"Referer": c.getHostLoginPageURL(),
	}
	body, err := c.getBytes(u.String(), extraHeaders)
	if err != nil {
		return nil, wrapPortalErr(err, "get status")
	}
	c.log.Debugf("status response: %s", string(body))

	info, err := parseLoginInfo(string(body), c.IP)
	if err != nil {
		return nil, err
	}
	if info.MAC != "" {
		c.MAC = NormalizeMAC(info.MAC)
		info.MAC = c.MAC
	}
	return info, nil
}

func parseLoginInfo(strLoginInfo, ip string) (*LoginInfo, error) {
	errMatch := reError.FindStringSubmatch(strLoginInfo)
	if len(errMatch) < 2 || errMatch[1] != "ok" {
		errInfo := "unknown"
		if len(errMatch) > 1 {
			errInfo = errMatch[1]
		}
		return nil, fmt.Errorf("%w: %s", ErrNotOnline, errInfo)
	}

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
		info.MAC = NormalizeMAC(m[1])
	}
	return info, nil
}

// FormatLoginInfo returns a human-readable status block.
func FormatLoginInfo(info *LoginInfo) string {
	return formatLoginInfo(info, "Login successful")
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
	padding := (41 - len(title)) / 2
	if padding < 0 {
		padding = 0
	}
	b.WriteString(strings.Repeat(" ", padding) + title + "\n")
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

func (c *Client) selfServiceLogOutInternal(quiet bool) error {
	if c.MAC == "" {
		if info, err := c.GetLoginInfo(); err == nil && info.MAC != "" {
			c.MAC = info.MAC
		}
	}
	if c.MAC == "" {
		return fmt.Errorf("%w: cannot kick sessions without device MAC", ErrMACUndetected)
	}

	if !quiet {
		c.log.Infof("logging into self-service to kick sessions matching MAC %s", c.MAC)
	}
	selfSvc := NewSelfServiceClient()
	selfSvc.SetLogger(c.log)
	if err := selfSvc.SetBind(c.Bind); err != nil {
		return err
	}
	if err := selfSvc.SSOLogin(c.Username); err != nil {
		return fmt.Errorf("SSO failed: %w", err)
	}
	kicked, err := selfSvc.KickAllByMAC(c.MAC)
	if err != nil {
		return fmt.Errorf("self-service kick failed: %w", err)
	}
	if kicked == 0 {
		return errors.New("failed to log out: no sessions kicked")
	}
	return nil
}

func (c *Client) logOutInternal(quiet bool) error {
	if c.IP == "" {
		if _, err := c.GetIP(); err != nil {
			return err
		}
	}

	if c.LogoutMode == "selfservice" {
		return c.selfServiceLogOutInternal(quiet)
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

	body, err := c.getBytes(u.String(), nil)
	if err != nil {
		return wrapPortalErr(err, "logout")
	}
	c.log.Debugf("logout response: %s", string(body))

	matches := reRes.FindStringSubmatch(string(body))
	if len(matches) > 1 && matches[1] == "ok" {
		return nil
	}

	if quiet {
		return errors.New("portal logout failed")
	}

	c.log.Infof("portal logout unavailable, trying self-service kick")
	return c.selfServiceLogOutInternal(quiet)
}

func (c *Client) getBytes(urlStr string, extraHeaders map[string]string) ([]byte, error) {
	resp, err := doRequest(c.httpClient, c.ctx(), http.MethodGet, urlStr, extraHeaders, nil, c.log)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
