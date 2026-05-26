package config

import (
	"os"
	"path/filepath"
)

const (
	userDirName  = "nwafu-srun"
	userFileName = "config.json"
)

// Location is kept for API compatibility; only user config is supported.
type Location string

const LocationUser Location = "user"

// Paths holds the user config file path.
type Paths struct {
	User string
}

// ResolvePaths returns the user config file path.
func ResolvePaths() (Paths, error) {
	userCfgDir, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, err
	}
	return Paths{
		User: filepath.Join(userCfgDir, userDirName, userFileName),
	}, nil
}

// DefaultPath returns the path used for load/save.
func DefaultPath(paths Paths) string {
	return paths.User
}

// PathFor returns the config file path (always user dir).
func PathFor(_ Location, paths Paths) string {
	return paths.User
}

// LoadCandidatePaths returns config paths to try, in order.
func LoadCandidatePaths(paths Paths, explicitPath string) []string {
	if explicitPath != "" {
		return []string{explicitPath}
	}
	if paths.User != "" {
		return []string{paths.User}
	}
	return nil
}
