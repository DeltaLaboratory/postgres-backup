package main

import (
	"context"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"

	"postgres-backup/internal/compress"
	"postgres-backup/internal/config"
	"postgres-backup/internal/dump"
	"postgres-backup/internal/upload/s3"
)

func CommandUpload() {
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

func CommandUploadSchedule() {
	if len(config.Loaded.Schedule) == 0 {
		log.Fatal().Msg("no schedule provided")
	}
	c := cron.New()
	for _, schedule := range config.Loaded.Schedule {
		if _, err := c.AddFunc(schedule, CommandUpload); err != nil {
			log.Fatal().Err(err).Str("schedule", schedule).Msg("failed to add schedule")
		}
		log.Info().Str("schedule", schedule).Msg("schedule added")
	}
	log.Info().Msg("starting cron")
	c.Run()
}
