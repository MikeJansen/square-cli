// Package config resolves square-cli configuration from (in order of
// increasing precedence): built-in defaults, the config file profile,
// environment variables, and command-line flags.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Environment names understood by the CLI.
const (
	EnvSandbox    = "sandbox"
	EnvProduction = "production"
)

// Base URLs for each Square environment.
const (
	BaseURLSandbox    = "https://connect.squareupsandbox.com"
	BaseURLProduction = "https://connect.squareup.com"
)

// Environment variable names.
const (
	envAccessToken = "SQUARE_ACCESS_TOKEN"
	envEnvironment = "SQUARE_ENVIRONMENT"
	envVersion     = "SQUARE_VERSION"
	envBaseURL     = "SQUARE_BASE_URL"
	envProfile     = "SQUARE_PROFILE"
	envConfig      = "SQUARE_CONFIG"
)

// Config is the fully-resolved configuration used to construct an API client.
type Config struct {
	AccessToken string
	Environment string
	APIVersion  string
	BaseURL     string
	Profile     string
}

// Profile is a single named configuration set stored in the config file.
type Profile struct {
	AccessToken string `yaml:"access_token,omitempty"`
	Environment string `yaml:"environment,omitempty"`
	APIVersion  string `yaml:"api_version,omitempty"`
	BaseURL     string `yaml:"base_url,omitempty"`
}

// File is the on-disk config file structure.
type File struct {
	CurrentProfile string             `yaml:"current_profile,omitempty"`
	Profiles       map[string]Profile `yaml:"profiles,omitempty"`
}

// Overrides carries values supplied on the command line. Empty strings mean
// "not set" and fall through to lower-precedence sources.
type Overrides struct {
	AccessToken string
	Environment string
	APIVersion  string
	BaseURL     string
	Profile     string
	ConfigPath  string
}

// DefaultConfigPath returns the default location of the config file, honoring
// SQUARE_CONFIG and XDG_CONFIG_HOME.
func DefaultConfigPath() string {
	if p := os.Getenv(envConfig); p != "" {
		return p
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "square-cli", "config.yaml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.yaml"
	}
	return filepath.Join(home, ".config", "square-cli", "config.yaml")
}

// LoadFile reads and parses the config file at path. A missing file is not an
// error and yields an empty File.
func LoadFile(path string) (File, error) {
	var f File
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return f, nil
		}
		return f, fmt.Errorf("read config %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &f); err != nil {
		return f, fmt.Errorf("parse config %s: %w", path, err)
	}
	return f, nil
}

// Resolve merges all configuration sources and returns the effective Config.
func Resolve(o Overrides) (Config, error) {
	cfg := Config{Environment: EnvProduction}

	path := o.ConfigPath
	if path == "" {
		path = DefaultConfigPath()
	}
	file, err := LoadFile(path)
	if err != nil {
		return cfg, err
	}

	// Determine active profile: flag > env > file default > "default".
	profileName := firstNonEmpty(o.Profile, os.Getenv(envProfile), file.CurrentProfile, "default")
	cfg.Profile = profileName

	// Layer 1: config file profile.
	if p, ok := file.Profiles[profileName]; ok {
		applyProfile(&cfg, p)
	} else if o.Profile != "" || os.Getenv(envProfile) != "" {
		// An explicitly requested profile that does not exist is an error.
		return cfg, fmt.Errorf("profile %q not found in %s", profileName, path)
	}

	// Layer 2: environment variables.
	applyProfile(&cfg, Profile{
		AccessToken: os.Getenv(envAccessToken),
		Environment: os.Getenv(envEnvironment),
		APIVersion:  os.Getenv(envVersion),
		BaseURL:     os.Getenv(envBaseURL),
	})

	// Layer 3: explicit flags.
	applyProfile(&cfg, Profile{
		AccessToken: o.AccessToken,
		Environment: o.Environment,
		APIVersion:  o.APIVersion,
		BaseURL:     o.BaseURL,
	})

	if cfg.Environment != EnvSandbox && cfg.Environment != EnvProduction {
		return cfg, fmt.Errorf("invalid environment %q (want %q or %q)", cfg.Environment, EnvSandbox, EnvProduction)
	}

	// Derive base URL from environment unless explicitly overridden.
	if cfg.BaseURL == "" {
		if cfg.Environment == EnvSandbox {
			cfg.BaseURL = BaseURLSandbox
		} else {
			cfg.BaseURL = BaseURLProduction
		}
	}

	return cfg, nil
}

func applyProfile(cfg *Config, p Profile) {
	if p.AccessToken != "" {
		cfg.AccessToken = p.AccessToken
	}
	if p.Environment != "" {
		cfg.Environment = p.Environment
	}
	if p.APIVersion != "" {
		cfg.APIVersion = p.APIVersion
	}
	if p.BaseURL != "" {
		cfg.BaseURL = p.BaseURL
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
