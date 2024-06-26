package cmd

import (
	"github.com/spf13/cobra"

	"github.com/DeltaLaboratory/postgres-backup/internal"
)

// backupCmd represents the backup command
var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup a PostgreSQL database",
	Long:  `Backup a PostgreSQL database one and now`,
	Run: func(cmd *cobra.Command, args []string) {
		internal.Backup()
	},
}

func init() {
	RootCmd.AddCommand(backupCmd)
}
