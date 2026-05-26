package srun

import "testing"

func TestLoginSucceeded(t *testing.T) {
	cases := []struct {
		res, err, msg string
		want          bool
	}{
		{"ok", "", "", true},
		{"", "ok", "", true},
		{"Authentication success,Welcome!", "", "", true},
		{"Authentication Success,Welcome!", "login_error", "", true},
		// Real-world: res/error say login_error but error_msg carries the
		// actual success marker.
		{"login_error", "login_error", "Authentication success,Welcome!", true},
		{"login_error", "login_error", "E2553: Password is error", false},
		{"login_error", "login_error", "E2553: Password is unsuccessful", false},
		{"unsuccessful", "", "", false},
		{"", "", "Welcome back!", true},
		{"", "", "authentication failed", false},
		{"E2553: Password is error", "login_error", "", false},
		{"", "", "", false},
		{"login_error", "login_error", "", false},
	}
	for _, c := range cases {
		got := loginSucceeded(c.res, c.err, c.msg)
		if got != c.want {
			t.Fatalf("loginSucceeded(%q,%q,%q)=%v want %v", c.res, c.err, c.msg, got, c.want)
		}
	}
}
