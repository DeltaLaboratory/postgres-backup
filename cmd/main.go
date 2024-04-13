package main

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"postgres-backup/internal/config"
	"postgres-backup/internal/dump"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	if err := config.LoadConfig(); err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	dumped, err := dump.Dump()
	if err != nil {
		log.Error().Err(err).Msg("failed to dump database")
		time.Sleep(5 * time.Hour)
		return
	}

	log.Info().Int("size", len(dumped)).Msg("database dumped")
}
