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
}

// Runtime holds merged settings for the current process.
type Runtime struct {
	File
	SourcePath string
	Paths      Paths
}

// CLIFlags are explicit command-line overrides (non-zero / set means override).
type CLIFlags struct {
	UsernameSet bool
	Username    string
	PasswordSet bool
	Password    string
	AcIDSet     bool
	AcID        string
	ForceSet    bool
	Force       bool
	BypassSet   bool
	Bypass      bool
	AllSet      bool
	All         bool
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
	paths, err := ResolvePaths()
	if err != nil {
		return &Runtime{Paths: paths}, nil
	}
	rt := &Runtime{Paths: paths}

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
	return &f, nil
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
	return out
}

// HasCredentials reports whether username and password are both set.
func (r Runtime) HasCredentials() bool {
	return r.Username != "" && r.Password != ""
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

// SaveNeverAskMarker writes minimal config to user dir disabling save prompts.
func SaveNeverAskMarker(paths Paths) error {
	if paths.User == "" {
		return errors.New("user config path unavailable")
	}
	return Save(paths.User, &File{
		Version:            CurrentVersion,
		SavePromptDisabled: true,
	})
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
