package schedule

import (
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/DeltaLaboratory/postgres-backup/internal"
	"github.com/DeltaLaboratory/postgres-backup/internal/config"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the backup schedule",
	Long:  `Run the backup schedule defined in the configuration file.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(config.Loaded.Schedule) == 0 {
			log.Fatal().Msg("no schedule provided")
		}

		c := cron.New()
		for _, schedule := range config.Loaded.Schedule {
			if id, err := c.AddFunc(schedule, internal.Backup); err != nil {
				log.Fatal().Err(err).Str("schedule", schedule).Msg("failed to add schedule")
			} else {
				log.Info().Str("schedule", schedule).Str("next", c.Entry(id).Next.String()).Msg("schedule added")
			}
		}
		log.Info().Msg("starting cron")
		c.Run()
	},
}

func init() {
	scheduleCmd.AddCommand(runCmd)
}
