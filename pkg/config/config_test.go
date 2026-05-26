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

func TestShouldOfferSavePrompt(t *testing.T) {
	rt := Runtime{
		File:       File{Username: "u", Password: "p", AutoAuth: true},
		SourcePath: "/cfg.json",
	}
	rt.Username = "u"
	rt.Password = "p"
	rt.AutoAuth = true
	if rt.ShouldOfferSavePrompt() {
		t.Fatal("should skip when auto-auth + matching creds")
	}
	rt.AutoAuth = false
	if !rt.ShouldOfferSavePrompt() {
		t.Fatal("should offer when creds match but auto_auth off")
	}
	rt.SavePromptDisabled = true
	if rt.ShouldOfferSavePrompt() {
		t.Fatal("should skip when never")
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

func TestFileForPersistAllOnlyWithBypass(t *testing.T) {
	rt := Runtime{File: File{Bypass: false, All: true, Username: "u", Password: "p"}}
	f := FileForPersist(rt, CLIFlags{}, File{})
	if f.All {
		t.Fatal("all should not persist without bypass")
	}
	rt.Bypass = true
	f2 := FileForPersist(rt, CLIFlags{BypassSet: true, AllSet: true}, File{})
	if !f2.All {
		t.Fatal("all should persist when -b and -a set on CLI")
	}
	f3 := FileForPersist(rt, CLIFlags{BypassSet: true}, File{All: true})
	if !f3.All {
		t.Fatal("all should be preserved from disk when saving bypass")
	}
}

func TestFileForPersistKeepsDiskPipelineWithoutCLI(t *testing.T) {
	disk := File{Force: true, Bypass: true, All: true, Username: "u", Password: "p"}
	rt := Runtime{File: disk}
	rt.Force = false
	rt.Bypass = false
	rt.All = false
	f := FileForPersist(rt, CLIFlags{}, disk)
	if !f.Force || !f.Bypass || !f.All {
		t.Fatalf("expected disk pipeline flags preserved, got %+v", f)
	}
}

func TestSanitizeClearsAllWithoutBypass(t *testing.T) {
	f := File{Bypass: false, All: true}
	f.Sanitize()
	if f.All {
		t.Fatal("expected all cleared")
	}
}

func TestSaveNeverAskMarkerPreservesCredentials(t *testing.T) {
	dir := t.TempDir()
	paths := Paths{User: filepath.Join(dir, "nwafu-srun", "config.json")}
	if err := Save(paths.User, &File{
		Version:  CurrentVersion,
		Username: "keep",
		Password: "secret",
	}); err != nil {
		t.Fatal(err)
	}
	if err := SaveNeverAskMarker(paths); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(LoadOptions{ExplicitPath: paths.User})
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Username != "keep" || loaded.Password != "secret" {
		t.Fatalf("credentials wiped: %+v", loaded.File)
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
