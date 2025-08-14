package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/DeltaLaboratory/postgres-backup/internal"
	"github.com/DeltaLaboratory/postgres-backup/internal/config"
	"github.com/DeltaLaboratory/postgres-backup/internal/storage/local"
	"github.com/DeltaLaboratory/postgres-backup/internal/storage/s3"
)

var (
	restoreBackupID   string
	restoreToDatabase string
	restoreListOnly   bool
	restoreStorage    string
)

// BackupEntry represents a backup with its metadata
type BackupEntry struct {
	Name         string
	LastModified time.Time
	Size         int64
	Source       string // "s3" or "local"
	Key          string // For S3: object key, For local: full file path
}

// restoreCmd represents the restore command
var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore PostgreSQL database from backup",
	Long: `Restore PostgreSQL database from a backup stored in S3 or local storage.

The restore command can list available backups and restore a selected backup to 
a PostgreSQL database. By default, it will show available backups for selection.

Examples:
  # List available backups from all configured storage backends
  postgres-backup restore --list

  # Restore the latest backup to the configured database
  postgres-backup restore --latest

  # Restore a specific backup by timestamp/filename
  postgres-backup restore --backup 2024-01-15T10:30:00

  # Restore to a different database
  postgres-backup restore --latest --to-database mydb_restored

  # Restore from specific storage backend only
  postgres-backup restore --list --storage s3
  postgres-backup restore --latest --storage local`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.Logger.With().Str("caller", "restore_cmd").Logger()

		// Validate storage configuration
		s3Configured := config.Loaded.Storage.S3 != nil
		localConfigured := config.Loaded.Storage.Local != nil

		if !s3Configured && !localConfigured {
			logger.Fatal().Msg("no storage backends configured - cannot perform restore operation")
		}

		// Filter storage backends if specified
		if restoreStorage != "" {
			switch restoreStorage {
			case "s3":
				if !s3Configured {
					logger.Fatal().Msg("S3 storage not configured")
				}
				localConfigured = false
			case "local":
				if !localConfigured {
					logger.Fatal().Msg("local storage not configured")
				}
				s3Configured = false
			default:
				logger.Fatal().Str("storage", restoreStorage).Msg("invalid storage backend specified, use 's3' or 'local'")
			}
		}

		// Handle list-only mode
		if restoreListOnly {
			listAvailableBackups(cmd.Context(), s3Configured, localConfigured)
			return
		}

		// Handle restore operation
		var selectedBackup *BackupEntry
		var err error

		if restoreBackupID == "latest" || (restoreBackupID == "" && len(args) == 0) {
			// Find latest backup
			selectedBackup, err = findLatestBackup(cmd.Context(), s3Configured, localConfigured)
			if err != nil {
				logger.Fatal().Err(err).Msg("failed to find latest backup")
			}
			if selectedBackup == nil {
				logger.Fatal().Msg("no backups found")
				return // This line will never execute, but helps staticcheck understand
			}
			logger.Info().Str("backup", selectedBackup.Name).Str("source", selectedBackup.Source).Msg("selected latest backup")
		} else if restoreBackupID != "" {
			// Find specific backup by ID
			selectedBackup, err = findBackupByID(cmd.Context(), restoreBackupID, s3Configured, localConfigured)
			if err != nil {
				logger.Fatal().Err(err).Str("backup_id", restoreBackupID).Msg("failed to find specified backup")
			}
			if selectedBackup == nil {
				logger.Fatal().Str("backup_id", restoreBackupID).Msg("backup not found")
				return // This line will never execute, but helps staticcheck understand
			}
			logger.Info().Str("backup", selectedBackup.Name).Str("source", selectedBackup.Source).Msg("found specified backup")
		} else {
			logger.Fatal().Msg("no backup specified - use --latest or --backup flag")
		}

		targetDb := getTargetDatabase()
		logger.Info().
			Str("backup", selectedBackup.Name).
			Str("source", selectedBackup.Source).
			Str("target_database", targetDb).
			Msg("starting restore operation")

		// Perform the restore
		if err := performRestore(cmd.Context(), selectedBackup, targetDb); err != nil {
			logger.Fatal().Err(err).Msg("restore operation failed")
		}

		logger.Info().
			Str("backup", selectedBackup.Name).
			Str("target_database", targetDb).
			Msg("restore operation completed successfully")
	},
}

func init() {
	restoreCmd.Flags().StringVar(&restoreBackupID, "backup", "", "specific backup to restore (timestamp or filename)")
	restoreCmd.Flags().BoolVar(&restoreListOnly, "list", false, "list available backups without restoring")
	restoreCmd.Flags().BoolVar(&restoreListOnly, "latest", false, "restore the most recent backup")
	restoreCmd.Flags().StringVar(&restoreToDatabase, "to-database", "", "target database name (defaults to configured database)")
	restoreCmd.Flags().StringVar(&restoreStorage, "storage", "", "storage backend to use: 's3' or 'local' (defaults to all configured)")

	// Mark backup and latest as mutually exclusive
	restoreCmd.MarkFlagsMutuallyExclusive("backup", "latest")

	RootCmd.AddCommand(restoreCmd)
}

func getTargetDatabase() string {
	if restoreToDatabase != "" {
		return restoreToDatabase
	}

	if config.Loaded.Postgres.Database != nil {
		return *config.Loaded.Postgres.Database
	}

	return "postgres" // default
}

func listS3Backups(ctx context.Context, logger zerolog.Logger) []BackupEntry {
	var backups []BackupEntry
	client, err := s3.CreateClient()
	if err != nil {
		logger.Error().Err(err).Msg("failed to create S3 client for listing backups")
		return backups
	}

	s3Backups, err := s3.ListBackups(ctx, client)
	if err != nil {
		logger.Error().Err(err).Msg("failed to list S3 backups")
		return backups
	}

	for _, backup := range s3Backups {
		backups = append(backups, BackupEntry{
			Name:         backup.Key,
			LastModified: backup.LastModified,
			Size:         backup.Size,
			Source:       "s3",
			Key:          backup.Key,
		})
	}
	logger.Info().Int("count", len(s3Backups)).Msg("found S3 backups")
	return backups
}

func listLocalBackups(logger zerolog.Logger) []BackupEntry {
	var backups []BackupEntry
	localBackups, err := local.ListBackups()
	if err != nil {
		logger.Error().Err(err).Msg("failed to list local backups")
		return backups
	}

	for _, backup := range localBackups {
		// Extract filename from full path for display
		filename := backup.Path
		if idx := strings.LastIndex(backup.Path, "/"); idx >= 0 {
			filename = backup.Path[idx+1:]
		}
		if idx := strings.LastIndex(filename, "\\"); idx >= 0 {
			filename = filename[idx+1:]
		}

		backups = append(backups, BackupEntry{
			Name:         filename,
			LastModified: backup.LastModified,
			Size:         backup.Size,
			Source:       "local",
			Key:          backup.Path,
		})
	}
	logger.Info().Int("count", len(localBackups)).Msg("found local backups")
	return backups
}

func listAvailableBackups(ctx context.Context, listS3, listLocal bool) {
	logger := log.Logger.With().Str("caller", "list_backups").Logger()

	logger.Info().
		Bool("s3", listS3).
		Bool("local", listLocal).
		Msg("listing available backups")

	var allBackups []BackupEntry

	// List S3 backups if configured and requested
	if listS3 && config.Loaded.Storage.S3 != nil {
		allBackups = append(allBackups, listS3Backups(ctx, logger)...)
	}

	// List local backups if configured and requested
	if listLocal && config.Loaded.Storage.Local != nil {
		allBackups = append(allBackups, listLocalBackups(logger)...)
	}

	// Sort all backups by modification time (newest first)
	sort.Slice(allBackups, func(i, j int) bool {
		return allBackups[i].LastModified.After(allBackups[j].LastModified)
	})

	// Display backups
	fmt.Fprintln(os.Stdout, "Available backups:")
	fmt.Fprintln(os.Stdout, "==================")

	if len(allBackups) == 0 {
		fmt.Fprintln(os.Stdout, "No backups found.")
		return
	}

	fmt.Fprintf(os.Stdout, "%-30s %-10s %-15s %s\n", "BACKUP NAME", "SOURCE", "SIZE", "CREATED")
	fmt.Fprintln(os.Stdout, strings.Repeat("-", 80))

	for _, backup := range allBackups {
		sizeStr := formatSize(backup.Size)
		timeStr := backup.LastModified.Format("2006-01-02 15:04")
		fmt.Fprintf(os.Stdout, "%-30s %-10s %-15s %s\n", backup.Name, backup.Source, sizeStr, timeStr)
	}

	fmt.Fprintf(os.Stdout, "\nTotal: %d backups\n", len(allBackups))
}

// formatSize formats a size in bytes to a human-readable string
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// getAllBackups gets all backups from configured storage backends
func getAllBackups(ctx context.Context, includeS3, includeLocal bool) ([]BackupEntry, error) {
	var allBackups []BackupEntry

	// Get S3 backups if configured and requested
	if includeS3 && config.Loaded.Storage.S3 != nil {
		client, err := s3.CreateClient()
		if err != nil {
			return nil, fmt.Errorf("failed to create S3 client: %w", err)
		}

		s3Backups, err := s3.ListBackups(ctx, client)
		if err != nil {
			return nil, fmt.Errorf("failed to list S3 backups: %w", err)
		}

		for _, backup := range s3Backups {
			allBackups = append(allBackups, BackupEntry{
				Name:         backup.Key,
				LastModified: backup.LastModified,
				Size:         backup.Size,
				Source:       "s3",
				Key:          backup.Key,
			})
		}
	}

	// Get local backups if configured and requested
	if includeLocal && config.Loaded.Storage.Local != nil {
		localBackups, err := local.ListBackups()
		if err != nil {
			return nil, fmt.Errorf("failed to list local backups: %w", err)
		}

		for _, backup := range localBackups {
			// Extract filename from full path for display
			filename := backup.Path
			if idx := strings.LastIndex(backup.Path, "/"); idx >= 0 {
				filename = backup.Path[idx+1:]
			}
			if idx := strings.LastIndex(filename, "\\"); idx >= 0 {
				filename = filename[idx+1:]
			}

			allBackups = append(allBackups, BackupEntry{
				Name:         filename,
				LastModified: backup.LastModified,
				Size:         backup.Size,
				Source:       "local",
				Key:          backup.Path,
			})
		}
	}

	// Sort by last modified time (newest first)
	sort.Slice(allBackups, func(i, j int) bool {
		return allBackups[i].LastModified.After(allBackups[j].LastModified)
	})

	return allBackups, nil
}

// findLatestBackup finds the most recent backup from configured storage backends
func findLatestBackup(ctx context.Context, includeS3, includeLocal bool) (*BackupEntry, error) {
	backups, err := getAllBackups(ctx, includeS3, includeLocal)
	if err != nil {
		return nil, err
	}

	if len(backups) == 0 {
		return nil, nil
	}

	return &backups[0], nil
}

// findBackupByID finds a specific backup by ID (timestamp or filename)
func findBackupByID(ctx context.Context, backupID string, includeS3, includeLocal bool) (*BackupEntry, error) {
	backups, err := getAllBackups(ctx, includeS3, includeLocal)
	if err != nil {
		return nil, err
	}

	for _, backup := range backups {
		// Check if backup name contains the ID (partial match for timestamp)
		if strings.Contains(backup.Name, backupID) {
			return &backup, nil
		}
		// Also check exact name match
		if backup.Name == backupID {
			return &backup, nil
		}
	}

	return nil, nil
}

// performRestore performs the actual restore operation
func performRestore(ctx context.Context, backup *BackupEntry, targetDatabase string) error {
	logger := log.Logger.With().
		Str("caller", "perform_restore").
		Str("backup", backup.Name).
		Str("source", backup.Source).
		Str("target_database", targetDatabase).
		Logger()

	logger.Info().Msg("starting restore operation")

	var backupReader io.ReadCloser
	var err error

	// Get backup data based on source
	switch backup.Source {
	case "s3":
		backupReader, err = s3.DownloadBackup(ctx, backup.Key)
		if err != nil {
			return fmt.Errorf("failed to download S3 backup: %w", err)
		}
	case "local":
		backupReader, err = local.OpenBackup(backup.Key)
		if err != nil {
			return fmt.Errorf("failed to open local backup: %w", err)
		}
	default:
		return fmt.Errorf("unsupported backup source: %s", backup.Source)
	}
	defer backupReader.Close()

	logger.Info().Msg("backup data retrieved, starting restore process")

	// Perform the restore
	err = internal.Restore(backupReader, targetDatabase, backup.Name)
	if err != nil {
		return fmt.Errorf("restore process failed: %w", err)
	}

	logger.Info().Msg("restore operation completed successfully")
	return nil
}
