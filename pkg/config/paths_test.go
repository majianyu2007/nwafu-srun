package config

import (
	"path/filepath"
	"testing"
)

func TestResolvePaths_userUnderConfigDir(t *testing.T) {
	paths, err := ResolvePaths()
	if err != nil {
		t.Fatal(err)
	}
	if paths.User == "" {
		t.Fatal("expected user path")
	}
	if filepath.Base(filepath.Dir(paths.User)) != userDirName {
		t.Fatalf("path = %s", paths.User)
	}
}

func TestLoadCandidatePaths_explicitWins(t *testing.T) {
	paths := Paths{User: "/home/u/.config/nwafu-srun/config.json"}
	got := LoadCandidatePaths(paths, "/custom.json")
	if len(got) != 1 || got[0] != "/custom.json" {
		t.Fatalf("got %v", got)
	}
}

func TestLoadCandidatePaths_userOnly(t *testing.T) {
	paths := Paths{User: "/home/u/.config/nwafu-srun/config.json"}
	got := LoadCandidatePaths(paths, "")
	if len(got) != 1 || got[0] != paths.User {
		t.Fatalf("got %v", got)
	}
}

func TestDefaultPath(t *testing.T) {
	p := Paths{User: "/a/b/config.json"}
	if DefaultPath(p) != p.User {
		t.Fatal("DefaultPath should return User")
	}
}
