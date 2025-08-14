package internal

import (
	"bytes"
	"context"
	"io"

	"github.com/rs/zerolog/log"

	"github.com/DeltaLaboratory/postgres-backup/internal/config"
	"github.com/DeltaLaboratory/postgres-backup/internal/storage/local"
	"github.com/DeltaLaboratory/postgres-backup/internal/storage/s3"
)

func Backup(ctx context.Context) {
	logger := log.Logger.With().Str("caller", "backup").Logger()

	// Get database name for logging context
	dbName := "unknown"
	if config.Loaded.Postgres.Database != nil {
		dbName = *config.Loaded.Postgres.Database
	}

	logger.Info().Str("database", dbName).Msg("starting database backup")

	process, err := Dump(ctx)
	if err != nil {
		logger.Error().Err(err).Str("database", dbName).Msg("failed to create database dump")
		return
	}

	if err := process.Start(); err != nil {
		logger.Error().Err(err).Str("database", dbName).Msg("failed to start pg_dump process")
		return
	}

	var reader io.Reader = process

	if config.Loaded.Compress != nil {
		logger.Info().Str("algorithm", config.Loaded.Compress.Algorithm).Int("compress_level", *config.Loaded.Compress.CompressLevel).Str("database", dbName).Msg("starting compression stream")
		reader, err = Compress(reader)
		if err != nil {
			logger.Error().Err(err).Str("database", dbName).Str("algorithm", config.Loaded.Compress.Algorithm).Msg("failed to compress database dump")
			return
		}
	}

	// Buffer the data if we need to upload to multiple storage backends
	var buffer *bytes.Buffer
	bothConfigured := config.Loaded.Storage.S3 != nil && config.Loaded.Storage.Local != nil

	if bothConfigured {
		// Read all data into buffer for multiple uploads
		logger.Info().Str("database", dbName).Msg("buffering backup data for multiple storage backends")
		buffer = &bytes.Buffer{}
		_, err = io.Copy(buffer, reader)
		if err != nil {
			logger.Error().Err(err).Str("database", dbName).Msg("failed to buffer backup data for storage")
			return
		}
	}

	// Upload to S3 if configured
	if config.Loaded.Storage.S3 != nil {
		var s3Reader io.Reader
		if bothConfigured {
			s3Reader = bytes.NewReader(buffer.Bytes())
		} else {
			s3Reader = reader
		}

		if err := s3.Upload(context.Background(), s3Reader); err != nil {
			logger.Error().Err(err).Str("database", dbName).Str("bucket", config.Loaded.Storage.S3.Bucket).Msg("failed to upload backup to S3")
		} else {
			logger.Info().Str("database", dbName).Str("bucket", config.Loaded.Storage.S3.Bucket).Msg("successfully uploaded backup to S3")
		}
	}

	// Upload to local storage if configured
	if config.Loaded.Storage.Local != nil {
		var localReader io.Reader
		if bothConfigured {
			localReader = bytes.NewReader(buffer.Bytes())
		} else {
			localReader = reader
		}

		if err := local.Upload(context.Background(), localReader); err != nil {
			logger.Error().Err(err).Str("database", dbName).Str("directory", config.Loaded.Storage.Local.Directory).Msg("failed to upload backup to local storage")
		} else {
			logger.Info().Str("database", dbName).Str("directory", config.Loaded.Storage.Local.Directory).Msg("successfully uploaded backup to local storage")
		}
	}

	if err := process.Wait(); err != nil {
		logger.Error().Err(err).Str("database", dbName).Msg("pg_dump process finished with error")
	}

	logger.Info().Str("database", dbName).Msg("database backup completed successfully")
}
