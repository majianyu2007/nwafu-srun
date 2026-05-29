package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const CurrentVersion = 1

// File is the on-disk configuration schema (version 1).
type File struct {
	Version            int    `json:"version"`
	Username           string `json:"username,omitempty"`
	Password           string `json:"password,omitempty"`
	AcID               string `json:"acid,omitempty"`
	AutoAuth           bool   `json:"auto_auth,omitempty"`
	Force              bool   `json:"force,omitempty"`
	Bypass             bool   `json:"bypass,omitempty"`
	All                bool   `json:"all,omitempty"`
	SavePromptDisabled bool   `json:"save_prompt_disabled,omitempty"`
	LogoutMode         string `json:"logout_mode,omitempty"`
}

// Runtime holds merged settings for the current process.
type Runtime struct {
	File
	SourcePath string
	Paths      Paths
}

// CLIFlags are explicit command-line overrides (non-zero / set means override).
type CLIFlags struct {
	UsernameSet   bool
	Username      string
	PasswordSet   bool
	Password      string
	AcIDSet       bool
	AcID          string
	ForceSet      bool
	Force         bool
	BypassSet     bool
	Bypass        bool
	AllSet        bool
	All           bool
	LogoutModeSet bool
	LogoutMode    string
}

// LoadOptions controls config discovery.
type LoadOptions struct {
	ExplicitPath string
	NoConfig     bool
}

var (
	ErrUnsupportedVersion = errors.New("unsupported config version")
	ErrConfigNotFound     = errors.New("config file not found")
)

// Load reads config from explicit path or the user config directory.
func Load(opts LoadOptions) (*Runtime, error) {
	paths, pathErr := ResolvePaths()
	rt := &Runtime{Paths: paths}
	if pathErr != nil && !opts.NoConfig {
		return rt, fmt.Errorf("user config directory unavailable: %w", pathErr)
	}

	if opts.NoConfig {
		return rt, nil
	}

	candidates := LoadCandidatePaths(paths, opts.ExplicitPath)

	for _, p := range candidates {
		if p == "" {
			continue
		}
		f, err := readFile(p)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) || errors.Is(err, ErrConfigNotFound) {
				continue
			}
			return nil, fmt.Errorf("read config %s: %w", p, err)
		}
		rt.File = *f
		rt.SourcePath = p
		return rt, nil
	}
	return rt, nil
}

func readFile(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, err
	}
	if len(data) == 0 {
		return nil, ErrConfigNotFound
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	if f.Version == 0 {
		f.Version = 1
	}
	if f.Version != CurrentVersion {
		return nil, fmt.Errorf("%w: got %d want %d", ErrUnsupportedVersion, f.Version, CurrentVersion)
	}
	f.Sanitize()
	return &f, nil
}

// Sanitize clears inconsistent flags after load or before save.
func (f *File) Sanitize() {
	if !f.Bypass {
		f.All = false
	}
}

// Merge applies priority: CLI > env > config file defaults.
func Merge(rt *Runtime, cli CLIFlags, envUser, envPass string) Runtime {
	out := *rt
	if out.AcID == "" {
		out.AcID = "1"
	}
	if envUser != "" && !cli.UsernameSet {
		out.Username = envUser
	}
	if envPass != "" && !cli.PasswordSet {
		out.Password = envPass
	}
	if cli.UsernameSet {
		out.Username = cli.Username
	}
	if cli.PasswordSet {
		out.Password = cli.Password
	}
	if cli.AcIDSet && cli.AcID != "" {
		out.AcID = cli.AcID
	}
	if cli.ForceSet {
		out.Force = cli.Force
	}
	if cli.BypassSet {
		out.Bypass = cli.Bypass
	}
	if cli.AllSet {
		out.All = cli.All
	}
	if cli.LogoutModeSet {
		out.LogoutMode = cli.LogoutMode
	}
	out.File.Sanitize()
	return out
}

// HasCredentials reports whether username and password are both set.
func (r Runtime) HasCredentials() bool {
	return r.Username != "" && r.Password != ""
}

// FileForPersist builds the on-disk config from runtime state.
//
// Pipeline flags (force/bypass/all) are taken from cli when the user set them
// on the command line; otherwise the values from fromDisk are kept so
// --save-config does not accidentally persist one-off merged flags.
func FileForPersist(rt Runtime, cli CLIFlags, fromDisk File) File {
	f := File{
		Version:            CurrentVersion,
		Username:           rt.Username,
		Password:           rt.Password,
		AcID:               rt.AcID,
		AutoAuth:           rt.AutoAuth,
		SavePromptDisabled: rt.SavePromptDisabled,
	}
	if cli.ForceSet {
		f.Force = rt.Force
	} else {
		f.Force = fromDisk.Force
	}
	if cli.BypassSet {
		f.Bypass = rt.Bypass
	} else {
		f.Bypass = fromDisk.Bypass
	}
	if f.Bypass {
		if cli.AllSet {
			f.All = rt.All
		} else {
			f.All = fromDisk.All
		}
	}
	if cli.LogoutModeSet {
		f.LogoutMode = rt.LogoutMode
	} else {
		f.LogoutMode = fromDisk.LogoutMode
	}
	f.Sanitize()
	return f
}

// PersistRuntime writes the user config file from runtime state.
func PersistRuntime(paths Paths, rt Runtime, cli CLIFlags, fromDisk File) error {
	if paths.User == "" {
		return errors.New("user config path unavailable")
	}
	f := FileForPersist(rt, cli, fromDisk)
	return Save(paths.User, &f)
}

// Save writes config to path, creating parent dirs and securing permissions.
func Save(path string, f *File) error {
	if f.Version == 0 {
		f.Version = CurrentVersion
	}
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	return secureFile(path)
}

// SaveNeverAskMarker sets save_prompt_disabled without removing other fields.
func SaveNeverAskMarker(paths Paths) error {
	if paths.User == "" {
		return errors.New("user config path unavailable")
	}
	f, err := readFile(paths.User)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			f = &File{Version: CurrentVersion}
		} else {
			return err
		}
	}
	f.SavePromptDisabled = true
	return Save(paths.User, f)
}

// ReenableSavePrompt clears save_prompt_disabled in the user config file if present.
func ReenableSavePrompt(paths Paths) error {
	if paths.User == "" {
		return errors.New("user config path unavailable")
	}
	f, err := readFile(paths.User)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	f.SavePromptDisabled = false
	return Save(paths.User, f)
}

// Redacted returns a copy safe for display (password masked).
func (f File) Redacted() File {
	out := f
	if out.Password != "" {
		out.Password = "********"
	}
	return out
}

// CredentialsMatch reports whether stored credentials equal the session.
func (f File) CredentialsMatch(username, password string) bool {
	return f.Username == username && f.Password == password
}

// ShouldOfferSavePrompt reports whether the interactive save-credentials dialog
// is still useful. Skip only when config already enables unattended login with
// the same credentials—not merely because the password matches on disk.
func (r Runtime) ShouldOfferSavePrompt() bool {
	if r.SavePromptDisabled || !r.HasCredentials() {
		return false
	}
	if r.SourcePath != "" && r.AutoAuth && r.File.CredentialsMatch(r.Username, r.Password) {
		return false
	}
	return true
}

// SavePromptSkipReason explains why the save dialog was not shown (empty if offered).
func (r Runtime) SavePromptSkipReason() string {
	if r.SavePromptDisabled {
		return "save prompt disabled; re-enable via Settings → Re-enable save prompt"
	}
	if !r.HasCredentials() {
		return ""
	}
	if r.SourcePath != "" && r.AutoAuth && r.File.CredentialsMatch(r.Username, r.Password) {
		return "credentials and auto-auth already configured at " + r.SourcePath
	}
	return ""
}
