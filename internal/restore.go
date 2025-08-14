package internal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/rs/zerolog/log"

	"github.com/DeltaLaboratory/postgres-backup/internal/config"
	"github.com/DeltaLaboratory/postgres-backup/internal/storage/local"
	"github.com/DeltaLaboratory/postgres-backup/internal/storage/s3"
)

// RestoreProcess represents a pg_restore process
type RestoreProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
}

// Start starts the pg_restore process
func (p *RestoreProcess) Start() error {
	var err error

	// Set up pipes for stdin, stdout, and stderr
	p.stdin, err = p.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	p.stdout, err = p.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	p.stderr, err = p.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start pg_restore process: %w", err)
	}

	return nil
}

// Wait waits for the pg_restore process to complete
func (p *RestoreProcess) Wait() error {
	logger := log.Logger.With().Str("caller", "restore_process_wait").Logger()

	// Close stdin to signal we're done sending data
	p.stdin.Close()

	// Read stdout and stderr concurrently
	stdoutChan := make(chan string, 1)
	stderrChan := make(chan string, 1)

	go func() {
		if p.stdout != nil {
			if output, err := io.ReadAll(p.stdout); err == nil {
				stdoutChan <- string(output)
			} else {
				stdoutChan <- ""
			}
			p.stdout.Close()
		} else {
			stdoutChan <- ""
		}
	}()

	go func() {
		if p.stderr != nil {
			if output, err := io.ReadAll(p.stderr); err == nil {
				stderrChan <- string(output)
			} else {
				stderrChan <- ""
			}
			p.stderr.Close()
		} else {
			stderrChan <- ""
		}
	}()

	// Wait for the process to complete
	err := p.cmd.Wait()

	// Collect output
	stdoutOutput := <-stdoutChan
	stderrOutput := <-stderrChan

	// Log outputs for debugging
	if stdoutOutput != "" {
		logger.Debug().Str("pg_restore_stdout", stdoutOutput).Msg("pg_restore stdout output")
	}

	if stderrOutput != "" {
		if err != nil {
			logger.Error().Str("pg_restore_stderr", stderrOutput).Msg("pg_restore error output")
		} else {
			logger.Debug().Str("pg_restore_stderr", stderrOutput).Msg("pg_restore stderr output")
		}
	}

	// Return enhanced error with pg_restore output
	if err != nil {
		if stderrOutput != "" {
			return fmt.Errorf("pg_restore failed: %w\npg_restore stderr: %s", err, stderrOutput)
		}
		return fmt.Errorf("pg_restore failed: %w", err)
	}

	return nil
}

// Write writes data to the pg_restore process stdin
func (p *RestoreProcess) Write(data []byte) (int, error) {
	if p.stdin == nil {
		return 0, errors.New("restore process is not started yet")
	}
	return p.stdin.Write(data)
}

// NewRestore creates a new pg_restore process for the specified database
func NewRestore(ctx context.Context, targetDatabase string) (*RestoreProcess, error) {
	process := new(RestoreProcess)

	argument := []string{
		"--format", "custom",
		"--host", config.Loaded.Postgres.Host,
		"--clean",         // Clean (drop) database objects before recreating them
		"--create",        // Create the database before restoring into it
		"--exit-on-error", // Exit on error, don't try to continue
		"--no-owner",      // Skip restoration of object ownership
		"--no-privileges", // Skip restoration of access privileges (grant/revoke commands)
		"--verbose",       // Verbose mode for detailed output
	}

	if config.Loaded.Postgres.Port != nil {
		argument = append(argument, "--port", strconv.Itoa(*config.Loaded.Postgres.Port))
	}

	if config.Loaded.Postgres.User != nil {
		argument = append(argument, "--username", *config.Loaded.Postgres.User)
	}

	// When using --create, connect to a maintenance database (postgres) instead of the target database
	// This avoids connection issues when the target database doesn't exist yet
	maintenanceDB := "postgres"
	if config.Loaded.Postgres.Database != nil && *config.Loaded.Postgres.Database != targetDatabase {
		maintenanceDB = *config.Loaded.Postgres.Database
	}
	argument = append(argument, "--dbname", maintenanceDB)

	process.cmd = exec.CommandContext(ctx, "pg_restore", argument...)
	if config.Loaded.Postgres.Password != nil {
		process.cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", *config.Loaded.Postgres.Password))
	}

	return process, nil
}

// Restore performs a complete restore operation from a backup reader to the target database
func Restore(backupReader io.Reader, targetDatabase, backupFilename string) error {
	ctx := context.Background()
	logger := log.Logger.With().
		Str("caller", "restore").
		Str("target_database", targetDatabase).
		Str("backup_filename", backupFilename).
		Logger()

	logger.Debug().Msg("starting restore operation")

	// Apply decompression if needed
	decompressedReader, err := Decompress(backupReader, backupFilename)
	if err != nil {
		return fmt.Errorf("failed to decompress backup: %w", err)
	}

	// Create pg_restore process
	restoreProcess, err := NewRestore(ctx, targetDatabase)
	if err != nil {
		return fmt.Errorf("failed to create restore process: %w", err)
	}

	// Start the restore process
	if err := restoreProcess.Start(); err != nil {
		return fmt.Errorf("failed to start restore process: %w", err)
	}

	logger.Debug().Msg("pg_restore process started, beginning data stream")

	// Stream backup data to pg_restore with better error handling
	bytesStreamed, err := io.Copy(restoreProcess, decompressedReader)
	if err != nil {
		logger.Error().
			Err(err).
			Int64("bytes_streamed", bytesStreamed).
			Msg("failed to stream backup data to pg_restore")

		// Try to get pg_restore error output for better diagnostics
		if waitErr := restoreProcess.Wait(); waitErr != nil {
			logger.Error().Err(waitErr).Msg("pg_restore process terminated with error during cleanup")
			return fmt.Errorf("failed to stream backup data to restore process: %w (pg_restore error: %v)", err, waitErr)
		} else {
			logger.Debug().Msg("pg_restore process terminated cleanly after streaming error")
		}
		return fmt.Errorf("failed to stream backup data to restore process: %w", err)
	}

	logger.Debug().
		Int64("bytes_streamed", bytesStreamed).
		Msg("backup data streaming completed, waiting for pg_restore to finish")

	// Wait for restore to complete
	if err := restoreProcess.Wait(); err != nil {
		logger.Error().Err(err).Msg("pg_restore process failed during completion")
		return fmt.Errorf("pg_restore process failed: %w", err)
	}

	logger.Debug().Msg("restore operation completed successfully")
	return nil
}

// BackupEntry represents a backup available for restore
type BackupEntry struct {
	Name      string
	Key       string
	Source    string // "s3" or "local"
	Timestamp time.Time
}

// ScheduledRestore performs a restore operation based on schedule configuration
func ScheduledRestore(ctx context.Context, scheduleConfig config.RestoreScheduleConfig) error {
	logger := log.Logger.With().
		Str("caller", "scheduled_restore").
		Str("target_database", scheduleConfig.TargetDatabase).
		Str("backup_selection", scheduleConfig.BackupSelection).
		Logger()

	logger.Info().Msg("starting scheduled restore operation")

	// Find the backup to restore based on selection strategy
	backup, err := findBackupForRestore(ctx, scheduleConfig)
	if err != nil {
		logger.Error().Err(err).Msg("failed to find backup for restore")
		return fmt.Errorf("failed to find backup for restore: %w", err)
	}

	if backup == nil {
		logger.Warn().Msg("no suitable backup found for restore")
		return errors.New("no suitable backup found for restore")
	}

	logger.Info().
		Str("selected_backup", backup.Name).
		Str("backup_source", backup.Source).
		Msg("selected backup for restore")

	// Perform the restore
	return performScheduledRestore(ctx, backup, scheduleConfig.TargetDatabase)
}

// findBackupForRestore finds the appropriate backup based on the schedule configuration
func findBackupForRestore(ctx context.Context, scheduleConfig config.RestoreScheduleConfig) (*BackupEntry, error) {
	backups, err := getAllBackupsForSchedule(ctx, scheduleConfig.ShouldIncludeS3(), scheduleConfig.ShouldIncludeLocal())
	if err != nil {
		return nil, err
	}

	if len(backups) == 0 {
		return nil, nil
	}

	switch scheduleConfig.BackupSelection {
	case "latest":
		return &backups[0], nil
	case "pattern":
		return findBackupByPattern(backups, *scheduleConfig.BackupPattern)
	case "specific":
		return findBackupBySpecificID(backups, *scheduleConfig.BackupID)
	default:
		return nil, fmt.Errorf("unsupported backup selection strategy: %s", scheduleConfig.BackupSelection)
	}
}

// getAllBackupsForSchedule retrieves all available backups from configured sources
func getAllBackupsForSchedule(ctx context.Context, includeS3, includeLocal bool) ([]BackupEntry, error) {
	var allBackups []BackupEntry

	// Get S3 backups if enabled
	if includeS3 && config.Loaded.Storage.S3 != nil {
		// Create S3 client
		client, err := minio.New(config.Loaded.Storage.S3.Endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(config.Loaded.Storage.S3.AccessKey, config.Loaded.Storage.S3.SecretKey, ""),
			Region: config.Loaded.Storage.S3.GetRegion(),
			Secure: true,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create S3 client: %w", err)
		}

		s3Backups, err := s3.ListBackups(ctx, client)
		if err != nil {
			return nil, fmt.Errorf("failed to list S3 backups: %w", err)
		}

		for _, backup := range s3Backups {
			// Parse timestamp from backup name (assuming format contains timestamp)
			timestamp, _ := parseTimestampFromBackupName(backup.Key)
			allBackups = append(allBackups, BackupEntry{
				Name:      backup.Key,
				Key:       backup.Key,
				Source:    "s3",
				Timestamp: timestamp,
			})
		}
	}

	// Get local backups if enabled
	if includeLocal && config.Loaded.Storage.Local != nil {
		localBackups, err := local.ListBackups()
		if err != nil {
			return nil, fmt.Errorf("failed to list local backups: %w", err)
		}

		for _, backup := range localBackups {
			// Parse timestamp from backup path - extract filename from full path
			filename := backup.Path
			if idx := strings.LastIndex(backup.Path, "\\"); idx >= 0 {
				filename = backup.Path[idx+1:]
			} else if idx := strings.LastIndex(backup.Path, "/"); idx >= 0 {
				filename = backup.Path[idx+1:]
			}
			timestamp, _ := parseTimestampFromBackupName(filename)
			allBackups = append(allBackups, BackupEntry{
				Name:      filename,
				Key:       backup.Path,
				Source:    "local",
				Timestamp: timestamp,
			})
		}
	}

	// Sort by timestamp (newest first)
	sort.Slice(allBackups, func(i, j int) bool {
		return allBackups[i].Timestamp.After(allBackups[j].Timestamp)
	})

	return allBackups, nil
}

// findBackupByPattern finds a backup matching the specified pattern
func findBackupByPattern(backups []BackupEntry, pattern string) (*BackupEntry, error) {
	for _, backup := range backups {
		if strings.Contains(backup.Name, pattern) {
			return &backup, nil
		}
	}
	return nil, nil
}

// findBackupBySpecificID finds a backup by specific ID
func findBackupBySpecificID(backups []BackupEntry, backupID string) (*BackupEntry, error) {
	for _, backup := range backups {
		if strings.Contains(backup.Name, backupID) || backup.Name == backupID {
			return &backup, nil
		}
	}
	return nil, nil
}

// parseTimestampFromBackupName attempts to parse timestamp from backup filename
func parseTimestampFromBackupName(filename string) (time.Time, error) {
	// Try common timestamp formats in backup filenames
	formats := []string{
		"2006-01-02_15-04-05",
		"20060102_150405",
		"2006-01-02T15:04:05",
		"20060102T150405",
	}

	for _, format := range formats {
		if timestamp, err := time.Parse(format, extractTimestampFromFilename(filename, format)); err == nil {
			return timestamp, nil
		}
	}

	// If no timestamp found, return zero time
	return time.Time{}, fmt.Errorf("could not parse timestamp from filename: %s", filename)
}

// extractTimestampFromFilename extracts timestamp portion from filename
func extractTimestampFromFilename(filename, _ string) string {
	// Simple extraction - look for timestamp-like patterns
	// This is a basic implementation and might need refinement
	parts := strings.Split(filename, "_")
	if len(parts) >= 2 {
		// Try to combine date and time parts
		return parts[len(parts)-2] + "_" + strings.Split(parts[len(parts)-1], ".")[0]
	}
	return filename
}

// performScheduledRestore performs the actual restore operation for scheduled restore
func performScheduledRestore(ctx context.Context, backup *BackupEntry, targetDatabase string) error {
	logger := log.Logger.With().
		Str("caller", "perform_scheduled_restore").
		Str("backup", backup.Name).
		Str("source", backup.Source).
		Str("target_database", targetDatabase).
		Logger()

	logger.Info().Msg("starting scheduled restore operation")

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
	err = Restore(backupReader, targetDatabase, backup.Name)
	if err != nil {
		return fmt.Errorf("restore process failed: %w", err)
	}

	logger.Info().Msg("scheduled restore operation completed successfully")
	return nil
}
