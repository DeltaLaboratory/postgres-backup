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
	Short: "Run the backup and restore schedules",
	Long:  `Run the backup and restore schedules defined in the configuration file.`,
	Run: func(cmd *cobra.Command, _ []string) {
		logger := log.Logger.With().Str("caller", "schedule_runner").Logger()

		backupCount := len(config.Loaded.Schedule)
		restoreCount := len(config.Loaded.RestoreSchedule)
		totalSchedules := backupCount + restoreCount

		if totalSchedules == 0 {
			logger.Fatal().Msg("no schedules configured - cannot start scheduler")
		}

		logger.Info().
			Int("backup_schedules", backupCount).
			Int("restore_schedules", restoreCount).
			Int("total_schedules", totalSchedules).
			Msg("initializing scheduler")

		c := cron.New()

		// Register backup schedules
		for _, schedule := range config.Loaded.Schedule {
			if _, err := c.AddFunc(schedule, func() { internal.Backup(cmd.Context()) }); err != nil {
				logger.Fatal().Err(err).
					Str("cron_expression", schedule).
					Msg("failed to register backup schedule - invalid cron expression")
			} else {
				logger.Info().
					Str("type", "backup").
					Str("cron_expression", schedule).
					Msg("schedule registered successfully")
			}
		}

		// Register restore schedules
		for _, restoreSchedule := range config.Loaded.RestoreSchedule {
			if !restoreSchedule.IsEnabled() {
				logger.Info().
					Str("type", "restore").
					Str("cron_expression", restoreSchedule.Cron).
					Str("target_database", restoreSchedule.TargetDatabase).
					Msg("restore schedule disabled, skipping")
				continue
			}

			// Create a closure to capture the restore schedule config
			scheduleConfig := restoreSchedule // Important: capture the value, not the reference
			if _, err := c.AddFunc(restoreSchedule.Cron, func() {
				if err := internal.ScheduledRestore(cmd.Context(), scheduleConfig); err != nil {
					log.Error().Err(err).
						Str("cron_expression", scheduleConfig.Cron).
						Str("target_database", scheduleConfig.TargetDatabase).
						Msg("scheduled restore failed")
				}
			}); err != nil {
				logger.Fatal().Err(err).
					Str("cron_expression", restoreSchedule.Cron).
					Str("target_database", restoreSchedule.TargetDatabase).
					Msg("failed to register restore schedule - invalid cron expression")
			} else {
				logger.Info().
					Str("type", "restore").
					Str("cron_expression", restoreSchedule.Cron).
					Str("target_database", restoreSchedule.TargetDatabase).
					Str("backup_selection", restoreSchedule.BackupSelection).
					Msg("schedule registered successfully")
			}
		}

		logger.Info().Msg("starting scheduler - waiting for scheduled jobs")
		c.Run()
	},
}

func init() {
	scheduleCmd.AddCommand(runCmd)
}
