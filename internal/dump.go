package internal

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/DeltaLaboratory/postgres-backup/internal/config"
)

type Process struct {
	cmd *exec.Cmd

	stdout io.ReadCloser
	done   chan struct{}
}

func (p *Process) Start() error {
	p.stdout, _ = p.cmd.StdoutPipe()
	if err := p.cmd.Start(); err != nil {
		return err
	}
	return nil
}

func (p *Process) Wait() error {
	err := p.cmd.Wait()
	p.stdout.Close()
	return err
}

func (p *Process) Read(pb []byte) (int, error) {
	if p.stdout == nil {
		return 0, fmt.Errorf("process is not started yet")
	}
	return p.stdout.Read(pb)
}

// TODO: dump multiple databases

func Dump() (*Process, error) {
	process := new(Process)

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

	process.cmd = exec.Command("pg_dump", argument...)
	if config.Loaded.Postgres.Password != nil {
		process.cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", *config.Loaded.Postgres.Password))
	}

	return process, nil
}
