package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/DeltaLaboratory/postgres-backup/internal/config"
	"github.com/DeltaLaboratory/postgres-backup/internal/storage/local"
	"github.com/DeltaLaboratory/postgres-backup/internal/storage/s3"
)

// retentionCmd represents the retention command
var retentionCmd = &cobra.Command{
	Use:   "retention",
	Short: "Manage backup retention policies",
	Long:  `Manage backup retention policies for cleaning up old backups.`,
}

// cleanupCmd represents the cleanup command
var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean up old backups based on retention policy",
	Long: `Clean up old backups based on the configured retention policy.
This command will remove backups that exceed the retention limits defined
in the configuration file (retention_days and/or retention_count).`,
	Run: func(cmd *cobra.Command, _ []string) {
		logger := log.Logger.With().Str("caller", "retention_cleanup_cmd").Logger()

		// Count configured storage backends
		backends := make([]string, 0, 2)
		if config.Loaded.Storage.S3 != nil {
			backends = append(backends, "S3")
		}
		if config.Loaded.Storage.Local != nil {
			backends = append(backends, "local")
		}

		if len(backends) == 0 {
			logger.Fatal().Msg("no storage backends configured - cannot perform retention cleanup")
		}

		logger.Info().
			Strs("storage_backends", backends).
			Msg("starting manual retention cleanup")

		successCount := 0

		// Run S3 retention cleanup if configured
		if config.Loaded.Storage.S3 != nil {
			if err := s3.CleanupRetention(cmd.Context()); err != nil {
				logger.Error().Err(err).
					Str("bucket", config.Loaded.Storage.S3.Bucket).
					Msg("S3 retention cleanup failed")
			} else {
				logger.Info().
					Str("bucket", config.Loaded.Storage.S3.Bucket).
					Msg("S3 retention cleanup completed successfully")
				successCount++
			}
		}

		// Run local retention cleanup if configured
		if config.Loaded.Storage.Local != nil {
			if err := local.CleanupRetention(cmd.Context()); err != nil {
				logger.Error().Err(err).
					Str("directory", config.Loaded.Storage.Local.Directory).
					Msg("local retention cleanup failed")
			} else {
				logger.Info().
					Str("directory", config.Loaded.Storage.Local.Directory).
					Msg("local retention cleanup completed successfully")
				successCount++
			}
		}

		logger.Info().
			Int("successful_backends", successCount).
			Int("total_backends", len(backends)).
			Msg("retention cleanup operation finished")
	},
}

func init() {
	retentionCmd.AddCommand(cleanupCmd)
	RootCmd.AddCommand(retentionCmd)
}
