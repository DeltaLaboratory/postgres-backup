package main

import (
	"context"

	"github.com/rs/zerolog/log"

	"postgres-backup/internal/compress"
	"postgres-backup/internal/config"
	"postgres-backup/internal/dump"
	"postgres-backup/internal/upload/s3"
)

func Upload() {
	dumped, err := dump.Dump()
	if err != nil {
		log.Error().Err(err).Msg("failed to dump database")
		return
	}

	log.Info().Int("size", len(dumped)).Msg("database dumped")

	if config.Loaded.CompressAlgorithm != nil {
		log.Info().Str("algorithm", *config.Loaded.CompressAlgorithm).Int("compress_level", *config.Loaded.CompressLevel).Msg("compressing dump")
		if compressed, err := compress.Compress(dumped); err != nil {
			log.Error().Err(err).Msg("failed to compress dump")
			return
		} else {
			log.Info().Int("size", len(compressed)).Msg("dump compressed")
			dumped = compressed
		}
	}

	if config.Loaded.Upload.S3 != nil {
		if err := s3.Upload(context.TODO(), dumped); err != nil {
			log.Error().Err(err).Msg("failed to upload dump to s3")
			return
		}
	}
}
