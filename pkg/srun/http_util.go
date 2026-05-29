package srun

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"time"
)

func newInsecureTransport() *http.Transport {
	return &http.Transport{
		// Explicitly do NOT use http.ProxyFromEnvironment.
		//
		// Both portal.nwafu.edu.cn and service.nwafu.edu.cn are campus-only
		// hosts; routing them through a user-level HTTP proxy almost always
		// makes them unreachable, and these endpoints must never leave the
		// campus network. Keeping Proxy nil also matches the zero-value, but
		// the explicit assignment documents the intent.
		Proxy:           nil,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // campus fallback uses IP with mismatched cert
	}
}

func newProbeClient() *http.Client {
	return &http.Client{
		Timeout:   ProbeTimeout,
		Transport: newInsecureTransport(),
	}
}

func newPortalHTTPClient(jar http.CookieJar) *http.Client {
	return &http.Client{
		Transport: newInsecureTransport(),
		Timeout:   PortalTimeout,
		Jar:       jar,
	}
}

func newSelfServiceHTTPClient(jar http.CookieJar) *http.Client {
	return &http.Client{
		Transport: newInsecureTransport(),
		Timeout:   SelfServiceTimeout,
		Jar:       jar,
	}
}

// probeURL returns true if GET url succeeds (any 2xx/3xx response).
func probeURL(client *http.Client, url string) bool {
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 400
}

// sleepFn is a test hook for delays (defaults to time.Sleep).
var sleepFn = time.Sleep

// sleep is used internally; tests may override sleepFn.
func sleep(d time.Duration) { sleepFn(d) }

// doRequest wraps http.NewRequestWithContext, sets User-Agent, logs the request, and executes it.
func doRequest(client *http.Client, ctx context.Context, method, urlStr string, extraHeaders map[string]string, body io.Reader, log logger) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, urlStr, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", DefaultUserAgent)
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	log.Debugf("%s %s", method, redactSensitive(urlStr))
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
