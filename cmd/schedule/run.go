package schedule

import (
	"context"

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
	Run: func(_ *cobra.Command, _ []string) {
		logger := log.Logger.With().Str("caller", "schedule_runner").Logger()

		if len(config.Loaded.Schedule) == 0 {
			logger.Fatal().Msg("no backup schedules configured - cannot start scheduler")
		}

		logger.Info().Int("schedule_count", len(config.Loaded.Schedule)).Msg("initializing backup scheduler")

		c := cron.New()
		for _, schedule := range config.Loaded.Schedule {
			if id, err := c.AddFunc(schedule, func() { internal.Backup(context.Background()) }); err != nil {
				logger.Fatal().Err(err).
					Str("cron_expression", schedule).
					Msg("failed to register backup schedule - invalid cron expression")
			} else {
				logger.Info().
					Str("cron_expression", schedule).
					Str("next_run", c.Entry(id).Next.String()).
					Msg("backup schedule registered successfully")
			}
		}

		logger.Info().Msg("starting backup scheduler - waiting for scheduled backup jobs")
		c.Run()
	},
}

func init() {
	scheduleCmd.AddCommand(runCmd)
}
