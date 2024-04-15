package main

import (
	"os"

	"github.com/robfig/cron/v3"
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
		Upload()
	case "upload-schedule":
		if config.Loaded.Schedule == nil || len(config.Loaded.Schedule) == 0 {
			log.Fatal().Msg("no schedule provided")
		}
		c := cron.New()
		for _, schedule := range config.Loaded.Schedule {
			_, err := c.AddFunc(schedule, Upload)
			if err != nil {
				log.Fatal().Err(err).Str("schedule", schedule).Msg("failed to add schedule")
			}
			log.Info().Str("schedule", schedule).Msg("schedule added")
		}
		log.Info().Msg("starting cron")
		c.Run()
	}
}
