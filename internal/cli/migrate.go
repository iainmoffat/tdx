package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
	"github.com/spf13/cobra"
)

// runStartupMigration performs the one-shot templates-per-profile migration.
// Errors are non-fatal: a warning is printed to stderr and the command proceeds.
func runStartupMigration() {
	paths, err := config.ResolvePaths()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tdx: templates migration warning: %v\n", err)
		return
	}
	store := config.NewProfileStore(paths)
	cfg, err := store.Load()
	if err != nil || cfg.DefaultProfile == "" {
		return // first run or unreadable config — skip silently
	}
	names := make([]string, 0, len(cfg.Profiles))
	for _, p := range cfg.Profiles {
		names = append(names, p.Name)
	}
	prompter := newCLIPrompter()
	result, err := draftsvc.Migrate(paths, names, cfg.DefaultProfile, prompter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tdx: templates migration warning: %v\n", err)
		return
	}
	if result.Migrated {
		fmt.Fprintf(os.Stderr, "tdx: migrated %d templates into profile %q.\n",
			result.FilesMoved, result.TargetProfile)
	}
}

// isNonInteractiveCommand returns true for subcommands that should never
// trigger a migration prompt (mcp serve, completion scripts, version).
func isNonInteractiveCommand(cmd *cobra.Command) bool {
	for _, seg := range strings.Fields(cmd.CommandPath()) {
		switch seg {
		case "mcp", "completion", "version":
			return true
		}
	}
	return false
}

// cliPrompter reads a y/yes confirmation from stdin when stdin is a TTY.
// When stdin is not a TTY it automatically answers no.
type cliPrompter struct{}

// Confirm prints question to stderr and reads a line from stdin.
// Returns false without error when stdin is not a terminal.
func (cliPrompter) Confirm(question string) (bool, error) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false, nil
	}
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		return false, nil // not a TTY — decline silently
	}
	fmt.Fprint(os.Stderr, question+" ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, nil
	}
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes", nil
}

// newCLIPrompter returns a Prompter backed by stdin/stderr.
func newCLIPrompter() draftsvc.Prompter { return cliPrompter{} }
