package srun

import (
	"fmt"
	"io"
	"os"
)

// logger provides optional diagnostic output (typically to stderr).
type logger interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
}

// nopLogger discards all log output.
type nopLogger struct{}

func (nopLogger) Debugf(string, ...any) {}
func (nopLogger) Infof(string, ...any)  {}
func (nopLogger) Warnf(string, ...any)  {}

// verboseLogger writes debug/info/warn lines to w when verbose is enabled.
type verboseLogger struct {
	verbose bool
	w       io.Writer
	prefix  string
}

// newVerboseLogger creates a logger that writes to stderr when verbose is true.
func newVerboseLogger(verbose bool, prefix string) *verboseLogger {
	return &verboseLogger{
		verbose: verbose,
		w:       os.Stderr,
		prefix:  prefix,
	}
}

func (l *verboseLogger) Debugf(format string, args ...any) {
	if l.verbose {
		fmt.Fprintf(l.w, "[%s] "+format+"\n", append([]any{l.prefix}, args...)...)
	}
}

func (l *verboseLogger) Infof(format string, args ...any) {
	if l.verbose {
		fmt.Fprintf(l.w, "[%s] "+format+"\n", append([]any{l.prefix}, args...)...)
	}
}

func (l *verboseLogger) Warnf(format string, args ...any) {
	if l.verbose {
		fmt.Fprintf(l.w, "[%s] WARN: "+format+"\n", append([]any{l.prefix}, args...)...)
		return
	}
	fmt.Fprintf(l.w, "WARN: "+format+"\n", args...)
}
