package srun

import (
	"net/http"
	"testing"
)

func TestBindOptionsNormalize(t *testing.T) {
	opts := (BindOptions{IP: " 192.0.2.10 ", Interface: " mv0 "}).normalized()
	if opts.IP != "192.0.2.10" || opts.Interface != "mv0" {
		t.Fatalf("unexpected normalized opts: %+v", opts)
	}
}

func TestBindOptionsLocalAddr(t *testing.T) {
	addr, err := (BindOptions{IP: "192.0.2.10"}).localAddr()
	if err != nil {
		t.Fatal(err)
	}
	if addr == nil || addr.IP.String() != "192.0.2.10" {
		t.Fatalf("unexpected local addr: %#v", addr)
	}
}

func TestBindOptionsLocalAddrRejectsInvalidIP(t *testing.T) {
	if _, err := (BindOptions{IP: "not-an-ip"}).localAddr(); err == nil {
		t.Fatal("expected invalid IP error")
	}
}

func TestClientSetBindRejectsInvalidIP(t *testing.T) {
	c := NewClient("user", "pass", "1")
	if err := c.SetBind(BindOptions{IP: "bad"}); err == nil {
		t.Fatal("expected invalid bind IP error")
	}
}

func TestSelfServiceSetBindRejectsInvalidIP(t *testing.T) {
	c := NewSelfServiceClient()
	if err := c.SetBind(BindOptions{IP: "bad"}); err == nil {
		t.Fatal("expected invalid bind IP error")
	}
}

func TestBindClientTransportPreservesClient(t *testing.T) {
	client := &http.Client{}
	if err := bindClientTransport(client, BindOptions{}); err != nil {
		t.Fatal(err)
	}
	if client.Transport == nil {
		t.Fatal("expected transport to be configured")
	}
}
