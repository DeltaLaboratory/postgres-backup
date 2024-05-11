package schedule

import (
	"github.com/spf13/cobra"

	"github.com/DeltaLaboratory/postgres-backup/cmd"
)

// scheduleCmd represents the schedule command
var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Run, manage and monitor backup schedules.",
	Long: `Run, manage and monitor backup schedules.
Schedules are defined in the configuration file.`,
}

func init() {
	cmd.RootCmd.AddCommand(scheduleCmd)
}
