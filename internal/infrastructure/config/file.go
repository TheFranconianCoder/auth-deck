package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/TheFranconianCoder/auth-deck/internal/core/entities"
	"github.com/TheFranconianCoder/auth-deck/internal/state"
)

type Config struct {
	Addr           string
	TokenStorePath string
	Catalog        *state.ProviderCatalog
}

type serverConfig struct {
	Addr       string `yaml:"addr"`
	TokenStore string `yaml:"token_store"`
}

type providerYAML struct {
	ClientID      string   `yaml:"client_id"`
	ClientSecret  string   `yaml:"client_secret"`
	AuthURL       string   `yaml:"auth_url"`
	TokenURL      string   `yaml:"token_url"`
	RedirectURI   string   `yaml:"redirect_uri"`
	Scopes        []string `yaml:"scopes"`
	BaseURL       string   `yaml:"base_url"`
	Flow          string   `yaml:"flow"`
	Resource      string   `yaml:"resource"`
	TokenPath     string   `yaml:"token_path"`
	TokenTypePath string   `yaml:"token_type_path"`
	ExpiresPath   string   `yaml:"expires_path"`
	PKCE          *bool    `yaml:"pkce"`
	Prompt        string   `yaml:"prompt"`
}

type configYAML struct {
	Server    serverConfig            `yaml:"server"`
	Providers map[string]providerYAML `yaml:"providers"`
}

var envVarRe = regexp.MustCompile(`\$\{([^}]+)\}`)

// Load reads and parses the YAML configuration, resolving ${ENV_VAR} references.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	expanded := envVarRe.ReplaceAllStringFunc(string(data), func(match string) string {
		varName := match[2 : len(match)-1]
		if val, ok := os.LookupEnv(varName); ok {
			return val
		}
		return match
	})

	var raw configYAML
	if err := yaml.Unmarshal([]byte(expanded), &raw); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	addr := raw.Server.Addr
	if addr == "" {
		addr = "127.0.0.1:9090"
	}

	tokenStorePath := raw.Server.TokenStore
	if tokenStorePath == "" {
		tokenStorePath = DefaultTokenStorePath()
	} else {
		tokenStorePath = ExpandPath(tokenStorePath)
	}

	providers := make(map[string]*entities.Provider, len(raw.Providers))
	for name, p := range raw.Providers {
		flow := entities.FlowType(p.Flow)
		if flow == "" {
			flow = entities.FlowClientCredentials
		}

		// PKCE defaults to on; it can be disabled explicitly for providers that
		// do not support the code_challenge parameters.
		pkce := true
		if p.PKCE != nil {
			pkce = *p.PKCE
		}

		providers[name] = &entities.Provider{
			Name:          name,
			ClientID:      p.ClientID,
			ClientSecret:  p.ClientSecret,
			AuthURL:       p.AuthURL,
			TokenURL:      p.TokenURL,
			RedirectURI:   p.RedirectURI,
			Scopes:        p.Scopes,
			BaseURL:       strings.TrimSuffix(p.BaseURL, "/"),
			Flow:          flow,
			Resource:      p.Resource,
			TokenPath:     defaultString(p.TokenPath, entities.DefaultTokenPath),
			TokenTypePath: defaultString(p.TokenTypePath, entities.DefaultTokenTypePath),
			ExpiresPath:   defaultString(p.ExpiresPath, entities.DefaultExpiresPath),
			PKCE:          pkce,
			Prompt:        p.Prompt,
		}
	}

	catalog := state.NewProviderCatalog(providerOrder([]byte(expanded)), providers)
	return &Config{
		Addr:           addr,
		TokenStorePath: tokenStorePath,
		Catalog:        catalog,
	}, nil
}

// DefaultTokenStorePath returns the OS-conventional location for the persisted
// token store: ~/.local/share/auth-deck on Linux (XDG), Application Support on
// macOS, and %AppData% on Windows.
func DefaultTokenStorePath() string {
	var base string
	switch runtime.GOOS {
	case "windows":
		if dir, err := os.UserConfigDir(); err == nil {
			base = dir
		}
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			base = filepath.Join(home, "Library", "Application Support")
		}
	default:
		if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
			base = dir
		} else if home, err := os.UserHomeDir(); err == nil {
			base = filepath.Join(home, ".local", "share")
		}
	}
	if base == "" {
		base = "."
	}
	return filepath.Join(base, "auth-deck", "tokens.json")
}

// DefaultConfigPath returns the OS-conventional location of the AuthDeck
// configuration file: $XDG_CONFIG_HOME/auth-deck/config.yaml on Linux,
// ~/Library/Application Support/auth-deck/config.yaml on macOS, and
// %AppData%\auth-deck\config.yaml on Windows.
func DefaultConfigPath() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "auth-deck", "config.yaml")
	}
	return "config.yaml"
}

// ExpandPath expands a leading ~ to the user's home directory.
func ExpandPath(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// providerOrder extracts provider names from the YAML in declaration order.
func providerOrder(data []byte) []string {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil || len(doc.Content) == 0 {
		return nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value != "providers" {
			continue
		}
		pm := root.Content[i+1]
		if pm.Kind != yaml.MappingNode {
			return nil
		}
		names := make([]string, 0, len(pm.Content)/2)
		for j := 0; j+1 < len(pm.Content); j += 2 {
			names = append(names, pm.Content[j].Value)
		}
		return names
	}
	return nil
}
