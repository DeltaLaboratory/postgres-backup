package main

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"postgres-backup/internal/config"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Str("caller", "postgres-backup").Logger()

	if err := config.LoadConfig(); err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	if len(os.Args) < 2 {
		log.Info().Msg("no command provided")
		log.Info().Msg("available commands: upload, upload-schedule")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "upload":
		CommandUpload()
	case "upload-schedule":
		CommandUploadSchedule()
	default:
		log.Error().Str("command", os.Args[1]).Msg("unknown command")
		os.Exit(1)
	}
}
