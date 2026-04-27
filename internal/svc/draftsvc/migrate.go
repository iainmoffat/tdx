package draftsvc

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/iainmoffat/tdx/internal/config"
)

// Prompter abstracts user confirmation so tests can supply silent prompters.
type Prompter interface {
	Confirm(question string) (bool, error)
}

// MigrateResult reports what happened during the templates-per-profile migration.
type MigrateResult struct {
	Migrated      bool
	FilesMoved    int
	TargetProfile string
}

// Migrate moves templates from the legacy ~/.config/tdx/templates/ directory
// into the active profile's per-profile templates directory. It is a no-op if
// the legacy directory has a .migrated marker, or if it does not exist.
//
// When more than one profile is configured, the prompter is asked which profile
// should own the templates. With a single profile, migration runs automatically
// and silently.
func Migrate(paths config.Paths, profiles []string, activeProfile string, prompter Prompter) (MigrateResult, error) {
	legacy := paths.LegacyTemplatesDir
	if legacy == "" {
		return MigrateResult{}, nil
	}
	if _, err := os.Stat(filepath.Join(legacy, ".migrated")); err == nil {
		return MigrateResult{}, nil
	}
	if _, err := os.Stat(legacy); os.IsNotExist(err) {
		return MigrateResult{}, nil
	} else if err != nil {
		return MigrateResult{}, err
	}

	target := activeProfile
	if len(profiles) > 1 {
		ok, err := prompter.Confirm(fmt.Sprintf(
			"Move legacy templates into profile %q? (run for each profile manually if not.) [y/N]",
			activeProfile))
		if err != nil {
			return MigrateResult{}, err
		}
		if !ok {
			return MigrateResult{}, nil
		}
	}

	targetDir := paths.ProfileTemplatesDir(target)
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		return MigrateResult{}, err
	}

	entries, err := os.ReadDir(legacy)
	if err != nil {
		return MigrateResult{}, err
	}

	moved := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		src := filepath.Join(legacy, e.Name())
		dst := filepath.Join(targetDir, e.Name())
		if err := moveFile(src, dst); err != nil {
			return MigrateResult{}, err
		}
		moved++
	}

	if err := os.WriteFile(filepath.Join(legacy, ".migrated"), []byte("ok\n"), 0o600); err != nil {
		return MigrateResult{}, err
	}
	return MigrateResult{Migrated: true, FilesMoved: moved, TargetProfile: target}, nil
}

// moveFile moves src to dst, falling back to a copy-then-delete if os.Rename
// fails (e.g. cross-device moves).
func moveFile(src, dst string) error {
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}
	if !errors.Is(err, syscall.EXDEV) {
		return err // surface non-cross-device errors directly
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(dst)
		return err
	}
	return os.Remove(src)
}
