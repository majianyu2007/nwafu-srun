//go:build !linux

package srun

import (
	"context"
	"fmt"
	"net"
)

func bindDialContext(opts BindOptions) (func(context.Context, string, string) (net.Conn, error), error) {
	if opts.Interface != "" {
		return nil, fmt.Errorf("bind interface is only supported on linux")
	}
	localAddr, err := opts.localAddr()
	if err != nil {
		return nil, err
	}
	dialer := &net.Dialer{LocalAddr: localAddr}
	return dialer.DialContext, nil
}
