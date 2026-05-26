package srun

import (
	"fmt"
	"io"
	"os"
)

// Logger provides optional diagnostic output (typically to stderr).
type Logger interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
}

// NopLogger discards all log output.
type NopLogger struct{}

func (NopLogger) Debugf(string, ...any) {}
func (NopLogger) Infof(string, ...any)  {}
func (NopLogger) Warnf(string, ...any)  {}

// VerboseLogger writes debug/info/warn lines to w when verbose is enabled.
type VerboseLogger struct {
	verbose bool
	w       io.Writer
	prefix  string
}

// NewVerboseLogger creates a logger that writes to stderr when verbose is true.
func NewVerboseLogger(verbose bool, prefix string) *VerboseLogger {
	return &VerboseLogger{
		verbose: verbose,
		w:       os.Stderr,
		prefix:  prefix,
	}
}

func (l *VerboseLogger) Debugf(format string, args ...any) {
	if l.verbose {
		fmt.Fprintf(l.w, "[%s] "+format+"\n", append([]any{l.prefix}, args...)...)
	}
}

func (l *VerboseLogger) Infof(format string, args ...any) {
	if l.verbose {
		fmt.Fprintf(l.w, "[%s] "+format+"\n", append([]any{l.prefix}, args...)...)
	}
}

func (l *VerboseLogger) Warnf(format string, args ...any) {
	fmt.Fprintf(l.w, "[%s] WARN: "+format+"\n", append([]any{l.prefix}, args...)...)
}
