package srun

import (
	"crypto/tls"
	"net/http"
	"time"
)

func newInsecureTransport() *http.Transport {
	return &http.Transport{
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
	return resp.StatusCode < 500
}

// Sleep is a test hook for delays (defaults to time.Sleep).
var Sleep = time.Sleep

// sleep is used internally; tests may override Sleep.
func sleep(d time.Duration) { Sleep(d) }
