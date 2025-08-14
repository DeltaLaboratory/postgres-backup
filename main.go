package main

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/DeltaLaboratory/postgres-backup/cmd"

	_ "github.com/DeltaLaboratory/postgres-backup/cmd/schedule"
)

func main() {
	//nolint:reassign // intended usage of zerolog global log
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Str("caller", "postgres-backup").Logger()

	cmd.Execute()
}
