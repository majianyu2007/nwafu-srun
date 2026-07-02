//go:build linux

package srun

import (
	"context"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

func bindControl(iface string) func(string, string, syscall.RawConn) error {
	return func(_, _ string, c syscall.RawConn) error {
		var controlErr error
		if err := c.Control(func(fd uintptr) {
			controlErr = unix.SetsockoptString(int(fd), unix.SOL_SOCKET, unix.SO_BINDTODEVICE, iface)
		}); err != nil {
			return err
		}
		return controlErr
	}
}

func bindDialContext(opts BindOptions) (func(context.Context, string, string) (net.Conn, error), error) {
	localAddr, err := opts.localAddr()
	if err != nil {
		return nil, err
	}

	dialer := &net.Dialer{LocalAddr: localAddr}
	if opts.Interface != "" {
		dialer.Control = bindControl(opts.Interface)
	}
	return dialer.DialContext, nil
}
