package srun

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Client handles communication with the Srun authentication portal
type Client struct {
	Username string
	Password string
	IP       string
	BaseURL  string

	httpClient *http.Client
}

// NewClient creates a new Srun Client instance
func NewClient(username, password string) *Client {
	// Custom transport to ignore TLS certificate errors when using fallback IP
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &Client{
		Username: username,
		Password: password,
		BaseURL:  "https://portal.nwafu.edu.cn",
		httpClient: &http.Client{
			Transport: tr,
			Timeout:   5 * time.Second,
		},
	}
}

// probeAndSetBaseURL checks if domain works, otherwise falls back to IP
func (c *Client) probeAndSetBaseURL() error {
	testURL := "https://portal.nwafu.edu.cn/srun_portal_pc?ac_id=1&theme=pro"
	_, err := c.httpClient.Get(testURL)
	if err != nil {
		fmt.Println("Warning: Could not connect to portal.nwafu.edu.cn, falling back to 172.26.8.11...")
		c.BaseURL = "http://172.26.8.11"
	}
	return nil
}

func (c *Client) getHostLoginPageURL() string {
	return c.BaseURL + "/srun_portal_pc?ac_id=1&theme=pro"
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

// GetIP fetches the current client IP from the portal
func (c *Client) GetIP() (string, error) {
	c.probeAndSetBaseURL()

	req, err := http.NewRequest("GET", c.getHostLoginPageURL(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.26 Safari/537.36")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get IP request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	re := regexp.MustCompile(`ip\s*:\s*"(.*?)"`)
	matches := re.FindStringSubmatch(string(body))
	if len(matches) > 1 {
		ip := matches[1]
		fmt.Printf("Your IP address: <%s>\n", ip)
		c.IP = ip
		return ip, nil
	}
	return "", fmt.Errorf("failed to get your IP address, please check your connection")
}

// GetChallenge gets the authentication token challenge
func (c *Client) GetChallenge() (string, error) {
	u, _ := url.Parse(c.getChallengeURL())
	q := u.Query()
	nowMs := strconv.FormatInt(time.Now().UnixNano()/1000000, 10)
	q.Set("callback", "jQuery11277455887669735664_"+nowMs)
	q.Set("username", c.Username)
	q.Set("ip", c.IP)
	q.Set("_", nowMs)
	u.RawQuery = q.Encode()

	req, _ := http.NewRequest("GET", u.String(), nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch token: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	re := regexp.MustCompile(`"challenge":"(.*?)"`)
	matches := re.FindStringSubmatch(string(body))
	if len(matches) > 1 {
		return matches[1], nil
	}
	return "", fmt.Errorf("failed to fetch token, please check your connection")
}

func (c *Client) getInfoString(challenge string) string {
	strInfo := fmt.Sprintf(`{"username":"%s","password":"%s","ip":"%s","acid":"1","enc_ver":"srun_bx1"}`, c.Username, c.Password, c.IP)
	encoded := jsBase64(xencode(strInfo, challenge))
	return "{SRBX1}" + encoded
}

func (c *Client) chksumAdd(challenge, md5Info, info string) string {
	str := challenge + c.Username
	str += challenge + md5Info
	str += challenge + "1" // acid
	str += challenge + c.IP
	str += challenge + "200" // n
	str += challenge + "1"   // vtype
	str += challenge + info
	return str
}

// LogIn attempts to login
func (c *Client) LogIn() {
	if c.IP == "" {
		_, err := c.GetIP()
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	challenge, err := c.GetChallenge()
	if err != nil {
		fmt.Println(err)
		return
	}

	md5Info := GetMD5(c.Password, challenge)
	md5Str := "{MD5}" + md5Info
	infoStr := c.getInfoString(challenge)
	chksumStr := GetSha1(c.chksumAdd(challenge, md5Info, infoStr))

	u, _ := url.Parse(c.getLogInURL())
	q := u.Query()
	q.Set("callback", "jQuery1124064")
	q.Set("action", "login")
	q.Set("username", c.Username)
	q.Set("password", md5Str)
	q.Set("ac_id", "1")
	q.Set("ip", c.IP)
	q.Set("info", infoStr)
	q.Set("chksum", chksumStr)
	q.Set("n", "200")
	q.Set("type", "1")
	q.Set("_", strconv.FormatInt(time.Now().UnixNano()/1000000, 10))
	u.RawQuery = q.Encode()

	req, _ := http.NewRequest("GET", u.String(), nil)
	req.Header.Set("Accept", "text/javascript, application/javascript, application/ecmascript, application/x-ecmascript, */*; q=0.01")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Cookie", "lang=zh-CN")
	req.Header.Set("Host", strings.TrimPrefix(strings.TrimPrefix(c.BaseURL, "https://"), "http://"))
	req.Header.Set("Referer", c.getHostLoginPageURL())
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		fmt.Println("-----------------------------------------")
		fmt.Println("Failed to authenticate (connection error)")
		fmt.Println("-----------------------------------------")
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	re := regexp.MustCompile(`"res":"(.*?)"`)
	matches := re.FindStringSubmatch(string(body))
	if len(matches) > 1 && matches[1] == "ok" {
		c.GetLoginInfo()
	} else {
		// Try to extract exact err_msg if available
		reErr := regexp.MustCompile(`"error_msg":"(.*?)"`)
		errMatches := reErr.FindStringSubmatch(string(body))
		errMsg := "unknown error"
		if len(errMatches) > 1 {
			errMsg = errMatches[1]
		}
		if len(matches) > 1 {
			errMsg = matches[1]
		}
		fmt.Println("-----------------------------------------")
		fmt.Printf("          Failed to authenticate         \n")
		fmt.Printf("          %s\n", errMsg)
		fmt.Println("-----------------------------------------")
	}
}

// GetLoginInfo gets current status and balance
func (c *Client) GetLoginInfo() {
	if c.IP == "" {
		fmt.Println("Cannot get status: IP is not yet resolved. Try logging in first.")
		return
	}

	u, _ := url.Parse(c.getLoginInfoURL())
	q := u.Query()
	q.Set("callback", "jQuery112402812915")
	q.Set("_", strconv.FormatInt(time.Now().Unix(), 10))
	u.RawQuery = q.Encode()

	req, _ := http.NewRequest("GET", u.String(), nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Referer", c.getHostLoginPageURL())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		fmt.Println("Failed to get authentication status (connection error)")
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	strLoginInfo := string(body)

	reErr := regexp.MustCompile(`"error":"(.*?)"`)
	errMatch := reErr.FindStringSubmatch(strLoginInfo)
	if len(errMatch) > 1 && errMatch[1] == "ok" {
		reUser := regexp.MustCompile(`"user_name":"(([a-zA-Z]|[0-9])*)"`)
		reBalance := regexp.MustCompile(`"user_balance":(.*?),`)
		reSumBytes := regexp.MustCompile(`"sum_bytes":(\d+),`)

		userName := "Unknown"
		if m := reUser.FindStringSubmatch(strLoginInfo); len(m) > 1 {
			userName = m[1]
		}
		balance := "0.00"
		if m := reBalance.FindStringSubmatch(strLoginInfo); len(m) > 1 {
			balance = m[1]
		}
		usedMB := 0.0
		if m := reSumBytes.FindStringSubmatch(strLoginInfo); len(m) > 1 {
			bytesVal, _ := strconv.ParseFloat(m[1], 64)
			usedMB = bytesVal / 1000000.0
		}

		fmt.Println("\n-----------------------------------------")
		fmt.Printf("%-20s-%20s\n", "Login successfully", "")
		fmt.Printf("%-20s-%20s\n", "     User name", userName)
		fmt.Printf("%-20s-%20s\n", "       Balance", balance)
		fmt.Printf("%-20s-%20.2f\n", "       Used MB", usedMB)
		fmt.Println("-----------------------------------------\n")
	} else {
		errInfo := "unknown"
		if len(errMatch) > 1 {
			errInfo = errMatch[1]
		}
		fmt.Printf("Failed to get authentication status: <%s>\n", errInfo)
	}
}

// LogOut attempts to log out
func (c *Client) LogOut() {
	if c.IP == "" {
		_, err := c.GetIP()
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	u, _ := url.Parse(c.getLogInURL())
	q := u.Query()
	q.Set("callback", "jQuery11240579338170130")
	q.Set("action", "logout")
	q.Set("ac_id", "1")
	q.Set("ip", c.IP)
	q.Set("username", c.Username)
	q.Set("_", strconv.FormatInt(time.Now().UnixNano()/1000000, 10))
	u.RawQuery = q.Encode()

	req, _ := http.NewRequest("GET", u.String(), nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		fmt.Println("-----------------------------------------")
		fmt.Println("Fail to logout (connection error)")
		fmt.Println("-----------------------------------------")
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	re := regexp.MustCompile(`"res":"(.*?)"`)
	matches := re.FindStringSubmatch(string(body))
	if len(matches) > 1 && matches[1] == "ok" {
		fmt.Println("-----------------------------------------")
		fmt.Println("           Logout successfully           ")
		fmt.Println("-----------------------------------------")
	} else {
		errMsg := "unknown"
		if len(matches) > 1 {
			errMsg = matches[1]
		}
		fmt.Println("-----------------------------------------")
		fmt.Println("             Fail to logout              ")
		fmt.Printf("             %s\n", errMsg)
		fmt.Println("-----------------------------------------")
	}
}
