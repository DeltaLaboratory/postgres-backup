package internal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"

	"github.com/rs/zerolog/log"

	"github.com/DeltaLaboratory/postgres-backup/internal/config"
)

// RestoreProcess represents a pg_restore process
type RestoreProcess struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
}

// Start starts the pg_restore process
func (p *RestoreProcess) Start() error {
	p.stdin, _ = p.cmd.StdinPipe()
	if err := p.cmd.Start(); err != nil {
		return err
	}
	return nil
}

// Wait waits for the pg_restore process to complete
func (p *RestoreProcess) Wait() error {
	p.stdin.Close()
	return p.cmd.Wait()
}

// Write writes data to the pg_restore process stdin
func (p *RestoreProcess) Write(data []byte) (int, error) {
	if p.stdin == nil {
		return 0, errors.New("restore process is not started yet")
	}
	return p.stdin.Write(data)
}

// NewRestore creates a new pg_restore process for the specified database
func NewRestore(ctx context.Context, targetDatabase string) (*RestoreProcess, error) {
	process := new(RestoreProcess)

	argument := []string{
		"--format", "custom",
		"--host", config.Loaded.Postgres.Host,
		"--clean",         // Clean (drop) database objects before recreating them
		"--create",        // Create the database before restoring into it
		"--exit-on-error", // Exit on error, don't try to continue
		"--no-owner",      // Skip restoration of object ownership
		"--no-privileges", // Skip restoration of access privileges (grant/revoke commands)
		"--verbose",       // Verbose mode for detailed output
	}

	if config.Loaded.Postgres.Port != nil {
		argument = append(argument, "--port", strconv.Itoa(*config.Loaded.Postgres.Port))
	}

	if config.Loaded.Postgres.User != nil {
		argument = append(argument, "--username", *config.Loaded.Postgres.User)
	}

	// Use the target database name
	argument = append(argument, "--dbname", targetDatabase)

	process.cmd = exec.CommandContext(ctx, "pg_restore", argument...)
	if config.Loaded.Postgres.Password != nil {
		process.cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", *config.Loaded.Postgres.Password))
	}

	return process, nil
}

// Restore performs a complete restore operation from a backup reader to the target database
func Restore(backupReader io.Reader, targetDatabase, backupFilename string) error {
	ctx := context.Background()

	// Apply decompression if needed
	decompressedReader, err := Decompress(backupReader, backupFilename)
	if err != nil {
		return fmt.Errorf("failed to decompress backup: %w", err)
	}

	// Create pg_restore process
	restoreProcess, err := NewRestore(ctx, targetDatabase)
	if err != nil {
		return fmt.Errorf("failed to create restore process: %w", err)
	}

	// Start the restore process
	if err := restoreProcess.Start(); err != nil {
		return fmt.Errorf("failed to start restore process: %w", err)
	}

	// Stream backup data to pg_restore
	_, err = io.Copy(restoreProcess, decompressedReader)
	if err != nil {
		if waitErr := restoreProcess.Wait(); waitErr != nil {
			log.Error().Err(waitErr).Msg("failed to wait for restore process cleanup")
		}
		return fmt.Errorf("failed to stream backup data to restore process: %w", err)
	}

	// Wait for restore to complete
	if err := restoreProcess.Wait(); err != nil {
		return fmt.Errorf("pg_restore process failed: %w", err)
	}

	return nil
}
