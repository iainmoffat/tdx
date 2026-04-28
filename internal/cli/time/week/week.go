package week

import "github.com/spf13/cobra"

// NewCmd returns the `tdx time week` command tree.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "week",
		Short: "Inspect weekly reports and manage week drafts",
	}
	cmd.AddCommand(newNewCmd())
	cmd.AddCommand(newShowCmd())
	cmd.AddCommand(newLockedCmd())
	cmd.AddCommand(newPullCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newDiffCmd())
	cmd.AddCommand(newPreviewCmd())
	cmd.AddCommand(newPushCmd())
	cmd.AddCommand(newDeleteCmd())
	cmd.AddCommand(newSetCmd())
	cmd.AddCommand(newNoteCmd())
	cmd.AddCommand(newEditCmd())
	cmd.AddCommand(newHistoryCmd())
	cmd.AddCommand(newCopyCmd())
	cmd.AddCommand(newRenameCmd())
	cmd.AddCommand(newResetCmd())
	cmd.AddCommand(newArchiveCmd())
	cmd.AddCommand(newUnarchiveCmd())
	cmd.AddCommand(newSnapshotCmd())
	cmd.AddCommand(newRestoreCmd())
	cmd.AddCommand(newPruneCmd())
	cmd.AddCommand(newRefreshCmd())
	return cmd
}
