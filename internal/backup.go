package internal

import (
	"context"
	"io"

	"github.com/rs/zerolog/log"

	"github.com/DeltaLaboratory/postgres-backup/internal/config"
	"github.com/DeltaLaboratory/postgres-backup/internal/storage/s3"
)

func Backup() {
	process, err := Dump()
	if err != nil {
		log.Error().Err(err).Msg("failed to dump database")
		return
	}

	if err := process.Start(); err != nil {
		log.Error().Err(err).Msg("failed to start backup process")
		return
	}

	var reader io.Reader = process

	if config.Loaded.Compress != nil {
		log.Info().Str("algorithm", config.Loaded.Compress.Algorithm).Int("compress_level", *config.Loaded.Compress.CompressLevel).Msg("start compress steam")
		reader, err = Compress(reader)
		if err != nil {
			log.Error().Err(err).Msg("failed to compress dump")
			return
		}
	}

	if config.Loaded.Storage.S3 != nil {
		if err := s3.Upload(context.Background(), reader); err != nil {
			log.Error().Err(err).Msg("failed to upload dump")
		}
	}

	if err := process.Wait(); err != nil {
		log.Error().Err(err).Msg("failed to run backup process")
	}

	log.Info().Msg("backup completed")
}
