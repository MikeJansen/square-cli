package config

import (
	"os"
	"path/filepath"
	"testing"
)

func isolateEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{envAccessToken, envEnvironment, envVersion, envBaseURL, envProfile, envConfig, "XDG_CONFIG_HOME"} {
		t.Setenv(k, "")
	}
}

func TestResolveDefaults(t *testing.T) {
	isolateEnv(t)
	t.Setenv(envConfig, filepath.Join(t.TempDir(), "missing.yaml"))

	cfg, err := Resolve(Overrides{AccessToken: "tok", Environment: EnvSandbox})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if cfg.AccessToken != "tok" {
		t.Errorf("access token = %q", cfg.AccessToken)
	}
	if cfg.BaseURL != BaseURLSandbox {
		t.Errorf("base url = %q, want %q", cfg.BaseURL, BaseURLSandbox)
	}
	if cfg.Profile != "default" {
		t.Errorf("profile = %q", cfg.Profile)
	}
}

func TestResolvePrecedence(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `current_profile: work
profiles:
  work:
    access_token: file-token
    environment: production
    api_version: "2020-01-01"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envConfig, path)
	t.Setenv(envAccessToken, "env-token") // env overrides file

	cfg, err := Resolve(Overrides{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if cfg.Profile != "work" {
		t.Errorf("profile = %q, want work", cfg.Profile)
	}
	if cfg.AccessToken != "env-token" {
		t.Errorf("access token = %q, want env-token (env should override file)", cfg.AccessToken)
	}
	if cfg.APIVersion != "2020-01-01" {
		t.Errorf("api version = %q", cfg.APIVersion)
	}
	if cfg.BaseURL != BaseURLProduction {
		t.Errorf("base url = %q", cfg.BaseURL)
	}

	// Flag overrides env.
	cfg, err = Resolve(Overrides{AccessToken: "flag-token"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if cfg.AccessToken != "flag-token" {
		t.Errorf("access token = %q, want flag-token", cfg.AccessToken)
	}
}

func TestResolveInvalidEnvironment(t *testing.T) {
	isolateEnv(t)
	t.Setenv(envConfig, filepath.Join(t.TempDir(), "missing.yaml"))
	if _, err := Resolve(Overrides{Environment: "staging"}); err == nil {
		t.Fatal("expected error for invalid environment")
	}
}

func TestResolveMissingProfile(t *testing.T) {
	isolateEnv(t)
	t.Setenv(envConfig, filepath.Join(t.TempDir(), "missing.yaml"))
	if _, err := Resolve(Overrides{Profile: "nope"}); err == nil {
		t.Fatal("expected error for missing explicit profile")
	}
}

func TestBaseURLOverride(t *testing.T) {
	isolateEnv(t)
	t.Setenv(envConfig, filepath.Join(t.TempDir(), "missing.yaml"))
	cfg, err := Resolve(Overrides{Environment: EnvSandbox, BaseURL: "http://localhost:9000"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if cfg.BaseURL != "http://localhost:9000" {
		t.Errorf("base url = %q", cfg.BaseURL)
	}
}
