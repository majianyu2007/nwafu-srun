package srun

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

// BindOptions controls which local IP/interface the portal and self-service
// HTTP clients should use when opening outbound connections.
type BindOptions struct {
	IP        string
	Interface string
}

func (o BindOptions) normalized() BindOptions {
	return BindOptions{
		IP:        strings.TrimSpace(o.IP),
		Interface: strings.TrimSpace(o.Interface),
	}
}

func (o BindOptions) localAddr() (*net.TCPAddr, error) {
	if o.IP == "" {
		return nil, nil
	}
	ip := net.ParseIP(o.IP)
	if ip == nil {
		return nil, fmt.Errorf("invalid bind IP %q", o.IP)
	}
	return &net.TCPAddr{IP: ip}, nil
}

func bindClientTransport(client *http.Client, opts BindOptions) error {
	transport, err := newInsecureTransport(opts)
	if err != nil {
		return err
	}
	client.Transport = transport
	return nil
}
