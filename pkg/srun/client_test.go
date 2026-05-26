package srun

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetIP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`ip: "10.1.2.3"`))
	}))
	defer srv.Close()

	c := NewClient("user", "pass", "1")
	c.BaseURL = srv.URL
	c.httpClient = srv.Client()

	ip, err := c.GetIP()
	if err != nil {
		t.Fatal(err)
	}
	if ip != "10.1.2.3" || c.IP != "10.1.2.3" {
		t.Fatalf("ip = %q", ip)
	}
}

func TestGetChallenge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"challenge":"abc123"}`))
	}))
	defer srv.Close()

	c := NewClient("user", "pass", "1")
	c.BaseURL = srv.URL
	c.IP = "10.0.0.1"
	c.httpClient = srv.Client()

	ch, err := c.GetChallenge()
	if err != nil {
		t.Fatal(err)
	}
	if ch != "abc123" {
		t.Fatalf("challenge = %q", ch)
	}
}

func TestParseLoginInfo_ok(t *testing.T) {
	body := `{"error":"ok","user_name":"testuser","user_balance":12.34,"sum_bytes":5000000,"user_mac":"aa:bb:cc:dd:ee:ff"}`
	info, err := parseLoginInfo(body, "10.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if info.Username != "testuser" || info.Balance != "12.34" || info.MAC != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("info = %+v", info)
	}
	if info.UsedMB < 4.9 || info.UsedMB > 5.1 {
		t.Fatalf("usedMB = %f", info.UsedMB)
	}
}

func TestParseLoginInfo_offline(t *testing.T) {
	_, err := parseLoginInfo(`{"error":"not_online"}`, "10.0.0.1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProbeURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	if !probeURL(srv.Client(), srv.URL) {
		t.Fatal("probe should succeed")
	}
}

func TestFormatLoginInfo(t *testing.T) {
	s := FormatLoginInfo(&LoginInfo{Username: "u", IP: "1.2.3.4", Balance: "1", UsedMB: 2.5, MAC: "aa:bb:cc:dd:ee:ff"})
	if s == "" || len(s) < 20 {
		t.Fatalf("format too short: %q", s)
	}
}

func TestFormatStatusInfo(t *testing.T) {
	s := FormatStatusInfo(&LoginInfo{Username: "u", IP: "1.2.3.4", Balance: "1", UsedMB: 2.5})
	if s == "" || len(s) < 20 {
		t.Fatalf("format too short: %q", s)
	}
	if !strings.Contains(s, "Current online status") {
		t.Fatalf("unexpected status title: %q", s)
	}
}
