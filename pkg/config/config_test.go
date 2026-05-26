package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	f := &File{
		Version:  CurrentVersion,
		Username: "user",
		Password: "pass",
		AcID:     "2",
		AutoAuth: true,
	}
	if err := Save(path, f); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(LoadOptions{ExplicitPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Username != "user" || loaded.Password != "pass" || !loaded.AutoAuth {
		t.Fatalf("got %+v", loaded.File)
	}
}

func TestMergePriority(t *testing.T) {
	rt := &Runtime{
		File: File{Username: "cfg", Password: "cfgpass", AcID: "1"},
	}
	cli := CLIFlags{UsernameSet: true, Username: "cli"}
	out := Merge(rt, cli, "", "")
	if out.Username != "cli" {
		t.Fatalf("username = %q", out.Username)
	}
	if out.Password != "cfgpass" {
		t.Fatalf("password should come from config when CLI password not set, got %q", out.Password)
	}
	cli2 := CLIFlags{PasswordSet: true, Password: "clipass"}
	out2 := Merge(rt, cli2, "env", "envpass")
	if out2.Password != "clipass" {
		t.Fatalf("password = %q", out2.Password)
	}
}

func TestCredentialsMatch(t *testing.T) {
	f := File{Username: "a", Password: "b"}
	if !f.CredentialsMatch("a", "b") {
		t.Fatal("should match")
	}
	if f.CredentialsMatch("a", "c") {
		t.Fatal("should not match")
	}
}

func TestRedacted(t *testing.T) {
	f := File{Password: "secret"}
	r := f.Redacted()
	if r.Password != "********" {
		t.Fatalf("redacted = %q", r.Password)
	}
}

func TestSaveNeverAskMarker(t *testing.T) {
	dir := t.TempDir()
	paths := Paths{User: filepath.Join(dir, "nwafu-srun", "config.json")}
	if err := SaveNeverAskMarker(paths); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(LoadOptions{ExplicitPath: paths.User})
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.SavePromptDisabled {
		t.Fatal("expected save_prompt_disabled")
	}
}

func TestUnsupportedVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte(`{"version":99}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(LoadOptions{ExplicitPath: path})
	if err == nil {
		t.Fatal("expected version error")
	}
}
