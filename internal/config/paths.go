package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Paths holds resolved filesystem locations used by tdx.
type Paths struct {
	Root            string
	ConfigFile      string
	CredentialsFile string
	TemplatesDir    string
}

// ResolvePaths determines the tdx config root and derived file locations.
// Precedence: TDX_CONFIG_HOME > XDG_CONFIG_HOME/tdx > $HOME/.config/tdx.
func ResolvePaths() (Paths, error) {
	root, err := resolveRoot()
	if err != nil {
		return Paths{}, err
	}
	return Paths{
		Root:            root,
		ConfigFile:      filepath.Join(root, "config.yaml"),
		CredentialsFile: filepath.Join(root, "credentials.yaml"),
		TemplatesDir:    filepath.Join(root, "templates"),
	}, nil
}

func resolveRoot() (string, error) {
	if v := os.Getenv("TDX_CONFIG_HOME"); v != "" {
		return v, nil
	}
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return filepath.Join(v, "tdx"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", "tdx"), nil
}

// EnsureRoot creates the config root directory if it does not exist.
func (p Paths) EnsureRoot() error {
	if err := os.MkdirAll(p.Root, 0o700); err != nil {
		return fmt.Errorf("create config root: %w", err)
	}
	return nil
}
