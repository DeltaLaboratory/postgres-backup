package dump

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/rs/zerolog/log"

	"postgres-backup/internal/config"
)

const MinVersion = 9
const MaxVersion = 16

func InstallClient(version int) error {
	if version < MinVersion || version > MaxVersion {
		return fmt.Errorf("unsupported version %d", version)
	}

	logger := log.Logger.With().Str("caller", "install_client").Logger()

	cmd := exec.Command("install_packages", fmt.Sprintf("postgresql-client-%d", version))

	if config.Loaded.IsVerbose() {
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()

		go func() {
			_, err := io.Copy(os.Stdout, stdout)
			if err != nil {
				logger.Error().Err(err).Msg("failed to copy stdout")
			}
		}()
		go func() {
			_, err := io.Copy(os.Stderr, stderr)
			if err != nil {
				logger.Error().Err(err).Msg("failed to copy stderr")
			}
		}()
	}

	logger.Info().Str("command", cmd.String()).Msg("installing dumper")

	err := cmd.Run()
	if err != nil {
		logger.Error().Err(err).Str("command", cmd.String()).Msg("failed to install postgres client")
		return err
	}

	logger.Info().Msg("postgres client installed")

	return nil
}

func Dump() ([]byte, error) {
	logger := log.Logger.With().Str("caller", "dump_database").Logger()

	if err := InstallClient(config.Loaded.Postgres.Version); err != nil {
		return nil, err
	}

	argument := []string{
		"--format", "custom",
		"--host", config.Loaded.Postgres.Host,
	}

	if config.Loaded.Postgres.Port != nil {
		argument = append(argument, "--port", fmt.Sprintf("%d", *config.Loaded.Postgres.Port))
	}

	if config.Loaded.Postgres.User != nil {
		argument = append(argument, "--username", *config.Loaded.Postgres.User)
	}

	if config.Loaded.Postgres.Database != nil {
		argument = append(argument, "--dbname", *config.Loaded.Postgres.Database)
	}

	cmd := exec.Command("pg_dump", argument...)
	if config.Loaded.Postgres.Password != nil {
		cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", *config.Loaded.Postgres.Password))
	}

	stdout, _ := cmd.StdoutPipe()

	logger.Info().Str("command", cmd.String()).Msg("dumping database")

	err := cmd.Start()
	if err != nil {
		logger.Error().Err(err).Str("command", cmd.String()).Msg("failed to dump database")
		return nil, err
	}

	backup, err := io.ReadAll(stdout)
	if err != nil {
		logger.Error().Err(err).Msg("failed to read backup")
		return nil, err
	}

	err = cmd.Wait()
	if err != nil {
		logger.Error().Err(err).Str("command", cmd.String()).Msg("failed to dump database")
		return nil, err
	}

	return backup, nil
}
