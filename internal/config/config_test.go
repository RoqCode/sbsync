package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReadsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg")
	content := []byte("SB_TOKEN=abc\nSOURCE_SPACE_ID=1\nTARGET_SPACE_ID=2\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Token != "abc" || cfg.SourceSpace != "1" || cfg.TargetSpace != "2" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("SB_TOKEN", "envtoken")
	cfg, err := Load(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Token != "envtoken" {
		t.Fatalf("expected token from env, got %q", cfg.Token)
	}
}

func TestSaveWritesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg")
	cfg := Config{Token: "abc", SourceSpace: "1", TargetSpace: "2"}
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	expected := "SB_TOKEN=abc\nSOURCE_SPACE_ID=1\nTARGET_SPACE_ID=2\n"
	if string(data) != expected {
		t.Fatalf("file content = %q, want %q", string(data), expected)
	}
}

func TestSaveRequiresToken(t *testing.T) {
	if err := Save("ignored", Config{}); err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestDefaultPathUsesHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	want := filepath.Join(dir, ".sbrc")
	if got := DefaultPath(); got != want {
		t.Fatalf("DefaultPath() = %q, want %q", got, want)
	}
}
