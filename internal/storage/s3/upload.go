package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/rs/zerolog/log"

	"github.com/DeltaLaboratory/postgres-backup/internal/config"
)

func Upload(ctx context.Context, reader io.Reader) error {
	logger := log.Logger.With().Str("caller", "s3_upload").Logger()

	if config.Loaded.Storage.S3 == nil {
		return errors.New("s3: config is not present")
	}

	client, err := minio.New(config.Loaded.Storage.S3.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.Loaded.Storage.S3.AccessKey, config.Loaded.Storage.S3.SecretKey, ""),
		Region: config.Loaded.Storage.S3.GetRegion(),
		Secure: true,
	})

	if err != nil {
		return fmt.Errorf("s3: failed to create client: %w", err)
	}

	if config.Loaded.IsVerbose() {
		client.TraceOn(log.Logger)
	}

	logger.Info().
		Str("endpoint", config.Loaded.Storage.S3.Endpoint).
		Str("bucket", config.Loaded.Storage.S3.Bucket).
		Msg("starting upload to S3")

	objectName := time.Now().Format("2006-01-02T15:04:05")

	if config.Loaded.Storage.S3.Prefix != nil {
		objectName = fmt.Sprintf("%s/%s", *config.Loaded.Storage.S3.Prefix, objectName)
	}

	if config.Loaded.Compress != nil {
		objectName = fmt.Sprintf("%s.%s", objectName, config.Loaded.Compress.Algorithm)
	}

	info, err := client.PutObject(ctx, config.Loaded.Storage.S3.Bucket, objectName, reader, -1, minio.PutObjectOptions{
		SendContentMd5: true,
	})
	if err != nil {
		return fmt.Errorf("s3: failed to store backup: %w", err)
	}

	logger.Info().
		Str("key", info.Key).
		Str("bucket", config.Loaded.Storage.S3.Bucket).
		Str("size", fmt.Sprintf("%d bytes", info.Size)).
		Msg("backup successfully uploaded to S3")

	// Run retention cleanup after successful upload
	if err := CleanupRetention(ctx); err != nil {
		logger.Warn().Err(err).Msg("failed to cleanup old S3 backups during retention policy enforcement")
	}

	return nil
}

// BackupInfo represents a backup file with its metadata
type BackupInfo struct {
	Key          string
	LastModified time.Time
	Size         int64
}

// ListBackups lists all backup files in the S3 bucket
func ListBackups(ctx context.Context, client *minio.Client) ([]BackupInfo, error) {
	var backups []BackupInfo
	prefix := ""

	if config.Loaded.Storage.S3.Prefix != nil {
		prefix = *config.Loaded.Storage.S3.Prefix
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
	}

	opts := minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	}

	for object := range client.ListObjects(ctx, config.Loaded.Storage.S3.Bucket, opts) {
		if object.Err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", object.Err)
		}

		// Skip directories
		if strings.HasSuffix(object.Key, "/") {
			continue
		}

		// Extract filename from the full key path
		filename := object.Key
		if idx := strings.LastIndex(object.Key, "/"); idx >= 0 {
			filename = object.Key[idx+1:]
		}

		// Only process files that match backup naming pattern (timestamp-based)
		// Format: 2006-01-02T15:04:05 with optional compression extension
		if len(filename) >= 19 && filename[4] == '-' && filename[7] == '-' && filename[10] == 'T' && filename[13] == ':' && filename[16] == ':' {
			backups = append(backups, BackupInfo{
				Key:          object.Key,
				LastModified: object.LastModified,
				Size:         object.Size,
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
func CleanupRetention(ctx context.Context) error {
	logger := log.Logger.With().Str("caller", "s3_retention_cleanup").Logger()

	if config.Loaded.Storage.S3 == nil {
		return nil // No S3 configuration
	}

	// Check if any retention policy is configured
	if !config.Loaded.Storage.S3.IsRetentionConfigured() {
		logger.Debug().Msg("no S3 retention policy configured, skipping cleanup")
		return nil // No retention policy configured
	}

	// Get effective retention days (handles both numeric and string periods)
	effectiveRetentionDays, err := config.Loaded.Storage.S3.GetEffectiveRetentionDays()
	if err != nil {
		return fmt.Errorf("failed to parse S3 retention period: %w", err)
	}

	retentionCount := config.Loaded.Storage.S3.RetentionCount

	// Log the retention policy being applied
	logEvent := logger.Info().Str("bucket", config.Loaded.Storage.S3.Bucket)
	if effectiveRetentionDays > 0 {
		logEvent = logEvent.Int("retention_days", effectiveRetentionDays)
		if config.Loaded.Storage.S3.RetentionPeriod != nil {
			logEvent = logEvent.Str("retention_period", *config.Loaded.Storage.S3.RetentionPeriod)
		}
	}
	if retentionCount != nil {
		logEvent = logEvent.Int("retention_count", *retentionCount)
	}
	logEvent.Msg("starting S3 retention cleanup")

	client, err := minio.New(config.Loaded.Storage.S3.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.Loaded.Storage.S3.AccessKey, config.Loaded.Storage.S3.SecretKey, ""),
		Region: config.Loaded.Storage.S3.GetRegion(),
		Secure: true,
	})
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %w", err)
	}

	backups, err := ListBackups(ctx, client)
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	var toDelete []string

	// Apply time-based retention
	if effectiveRetentionDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -effectiveRetentionDays)
		for _, backup := range backups {
			if backup.LastModified.Before(cutoff) {
				toDelete = append(toDelete, backup.Key)
			}
		}
	}

	// Apply count-based retention
	if retentionCount != nil && len(backups) > *retentionCount {
		for i := *retentionCount; i < len(backups); i++ {
			// Only add to delete list if not already marked for deletion
			found := false
			for _, key := range toDelete {
				if key == backups[i].Key {
					found = true
					break
				}
			}
			if !found {
				toDelete = append(toDelete, backups[i].Key)
			}
		}
	}

	// Delete the marked backups
	for _, key := range toDelete {
		err := client.RemoveObject(ctx, config.Loaded.Storage.S3.Bucket, key, minio.RemoveObjectOptions{})
		if err != nil {
			logger.Error().Err(err).
				Str("key", key).
				Str("bucket", config.Loaded.Storage.S3.Bucket).
				Msg("failed to delete S3 backup during retention cleanup")
		} else {
			logger.Info().
				Str("key", key).
				Str("bucket", config.Loaded.Storage.S3.Bucket).
				Msg("deleted old S3 backup")
		}
	}

	if len(toDelete) > 0 {
		logger.Info().
			Int("deleted_count", len(toDelete)).
			Str("bucket", config.Loaded.Storage.S3.Bucket).
			Msg("S3 retention cleanup completed successfully")
	} else {
		logger.Info().
			Str("bucket", config.Loaded.Storage.S3.Bucket).
			Msg("S3 retention cleanup completed - no backups to delete")
	}

	return nil
}

// CreateClient creates and configures an S3 client
func CreateClient() (*minio.Client, error) {
	if config.Loaded.Storage.S3 == nil {
		return nil, errors.New("s3: config is not present")
	}

	client, err := minio.New(config.Loaded.Storage.S3.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.Loaded.Storage.S3.AccessKey, config.Loaded.Storage.S3.SecretKey, ""),
		Region: config.Loaded.Storage.S3.GetRegion(),
		Secure: true,
	})

	if err != nil {
		return nil, fmt.Errorf("s3: failed to create client: %w", err)
	}

	if config.Loaded.IsVerbose() {
		client.TraceOn(log.Logger)
	}

	return client, nil
}

// DownloadBackup downloads a backup file from S3 and returns an io.ReadCloser
func DownloadBackup(ctx context.Context, backupKey string) (io.ReadCloser, error) {
	logger := log.Logger.With().Str("caller", "s3_download").Logger()

	if config.Loaded.Storage.S3 == nil {
		return nil, errors.New("s3: config is not present")
	}

	client, err := CreateClient()
	if err != nil {
		return nil, err
	}

	logger.Info().
		Str("key", backupKey).
		Str("bucket", config.Loaded.Storage.S3.Bucket).
		Msg("downloading backup from S3")

	object, err := client.GetObject(ctx, config.Loaded.Storage.S3.Bucket, backupKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("s3: failed to get backup object: %w", err)
	}

	// Get object info to log download details
	stat, err := object.Stat()
	if err != nil {
		object.Close()
		return nil, fmt.Errorf("s3: failed to stat backup object: %w", err)
	}

	logger.Info().
		Str("key", backupKey).
		Str("bucket", config.Loaded.Storage.S3.Bucket).
		Str("size", fmt.Sprintf("%d bytes", stat.Size)).
		Msg("successfully started backup download from S3")

	return object, nil
}
