package local

// File updated to remove .sql suffix from backup names

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/DeltaLaboratory/postgres-backup/internal/config"
)

// Upload saves the backup to local storage
func Upload(ctx context.Context, reader io.Reader) error {
	logger := log.Logger.With().Str("caller", "local_upload").Logger()

	if config.Loaded.Storage.Local == nil {
		return errors.New("local: config is not present")
	}

	// Ensure directory exists
	if err := os.MkdirAll(config.Loaded.Storage.Local.Directory, 0755); err != nil {
		return fmt.Errorf("local: failed to create directory: %w", err)
	}

	logger.Info().
		Str("directory", config.Loaded.Storage.Local.Directory).
		Msg("starting upload to local storage")

	filename := time.Now().Format("2006-01-02T15:04:05")

	if config.Loaded.Compress != nil {
		filename = fmt.Sprintf("%s.%s", filename, config.Loaded.Compress.Algorithm)
	}
	filepath := filepath.Join(config.Loaded.Storage.Local.Directory, filename)

	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("local: failed to create file: %w", err)
	}
	defer file.Close()

	bytesWritten, err := io.Copy(file, reader)
	if err != nil {
		return fmt.Errorf("local: failed to write backup: %w", err)
	}

	logger.Info().
		Str("file", filepath).
		Str("directory", config.Loaded.Storage.Local.Directory).
		Str("size", fmt.Sprintf("%d bytes", bytesWritten)).
		Msg("backup successfully uploaded to local storage")

	// Run retention cleanup after successful upload
	if err := CleanupRetention(ctx); err != nil {
		logger.Warn().Err(err).Msg("failed to cleanup old local backups during retention policy enforcement")
	}

	return nil
}

// BackupInfo represents a local backup file with its metadata
type BackupInfo struct {
	Path         string
	LastModified time.Time
	Size         int64
}

// ListBackups lists all backup files in the local directory
func ListBackups() ([]BackupInfo, error) {
	var backups []BackupInfo

	entries, err := os.ReadDir(config.Loaded.Storage.Local.Directory)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process files that match backup naming pattern (timestamp-based)
		// Format: 2006-01-02T15:04:05 with optional compression extension
		filename := entry.Name()
		if len(filename) >= 19 && filename[4] == '-' && filename[7] == '-' && filename[10] == 'T' && filename[13] == ':' && filename[16] == ':' {
			fullPath := filepath.Join(config.Loaded.Storage.Local.Directory, entry.Name())
			info, err := entry.Info()
			if err != nil {
				continue // Skip files we can't stat
			}

			backups = append(backups, BackupInfo{
				Path:         fullPath,
				LastModified: info.ModTime(),
				Size:         info.Size(),
			})
		}
	}

	// Sort by last modified time (newest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].LastModified.After(backups[j].LastModified)
	})

	return backups, nil
}

// CleanupRetention removes old backups based on retention policy
func CleanupRetention(_ context.Context) error {
	logger := log.Logger.With().Str("caller", "local_retention_cleanup").Logger()

	if config.Loaded.Storage.Local == nil {
		return nil // No local configuration
	}

	// Check if any retention policy is configured
	if !config.Loaded.Storage.Local.IsRetentionConfigured() {
		logger.Debug().Msg("no local retention policy configured, skipping cleanup")
		return nil // No retention policy configured
	}

	// Get effective retention days (handles both numeric and string periods)
	effectiveRetentionDays, err := config.Loaded.Storage.Local.GetEffectiveRetentionDays()
	if err != nil {
		return fmt.Errorf("failed to parse local retention period: %w", err)
	}

	retentionCount := config.Loaded.Storage.Local.RetentionCount

	// Log the retention policy being applied
	logEvent := logger.Info().Str("directory", config.Loaded.Storage.Local.Directory)
	if effectiveRetentionDays > 0 {
		logEvent = logEvent.Int("retention_days", effectiveRetentionDays)
		if config.Loaded.Storage.Local.RetentionPeriod != nil {
			logEvent = logEvent.Str("retention_period", *config.Loaded.Storage.Local.RetentionPeriod)
		}
	}
	if retentionCount != nil {
		logEvent = logEvent.Int("retention_count", *retentionCount)
	}
	logEvent.Msg("starting local retention cleanup")

	backups, err := ListBackups()
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	var toDelete []string

	// Apply time-based retention
	if effectiveRetentionDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -effectiveRetentionDays)
		for _, backup := range backups {
			if backup.LastModified.Before(cutoff) {
				toDelete = append(toDelete, backup.Path)
			}
		}
	}

	// Apply count-based retention
	if retentionCount != nil && len(backups) > *retentionCount {
		for i := *retentionCount; i < len(backups); i++ {
			// Only add to delete list if not already marked for deletion
			found := false
			for _, path := range toDelete {
				if path == backups[i].Path {
					found = true
					break
				}
			}
			if !found {
				toDelete = append(toDelete, backups[i].Path)
			}
		}
	}

	// Delete the marked backups
	for _, path := range toDelete {
		err := os.Remove(path)
		if err != nil {
			logger.Error().Err(err).
				Str("path", path).
				Str("directory", config.Loaded.Storage.Local.Directory).
				Msg("failed to delete local backup during retention cleanup")
		} else {
			logger.Info().
				Str("path", path).
				Str("directory", config.Loaded.Storage.Local.Directory).
				Msg("deleted old local backup")
		}
	}

	if len(toDelete) > 0 {
		logger.Info().
			Int("deleted_count", len(toDelete)).
			Str("directory", config.Loaded.Storage.Local.Directory).
			Msg("local retention cleanup completed successfully")
	} else {
		logger.Info().
			Str("directory", config.Loaded.Storage.Local.Directory).
			Msg("local retention cleanup completed - no backups to delete")
	}

	return nil
}

// OpenBackup opens a local backup file and returns an io.ReadCloser
func OpenBackup(backupPath string) (io.ReadCloser, error) {
	logger := log.Logger.With().Str("caller", "local_open_backup").Logger()

	if config.Loaded.Storage.Local == nil {
		return nil, errors.New("local: config is not present")
	}

	logger.Info().
		Str("path", backupPath).
		Msg("opening local backup file")

	// Verify the file exists and get its info
	fileInfo, err := os.Stat(backupPath)
	if err != nil {
		return nil, fmt.Errorf("local: failed to stat backup file: %w", err)
	}

	if fileInfo.IsDir() {
		return nil, errors.New("local: backup path is a directory, not a file")
	}

	// Open the backup file
	file, err := os.Open(backupPath)
	if err != nil {
		return nil, fmt.Errorf("local: failed to open backup file: %w", err)
	}

	logger.Info().
		Str("path", backupPath).
		Str("size", fmt.Sprintf("%d bytes", fileInfo.Size())).
		Msg("successfully opened local backup file")

	return file, nil
}
