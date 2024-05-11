package config

import (
	"os"

	"github.com/hashicorp/hcl/v2/hclsimple"

	"github.com/DeltaLaboratory/postgres-backup/internal/config/storage"
)

var Loaded *Config

type Config struct {
	Postgres PostgresConfig  `hcl:"postgres,block"`
	Storage  storage.Storage `hcl:"storage,block"`
	Compress *CompressConfig `hcl:"compress,block"`

	Schedule []string `hcl:"schedule"`

	Verbose *bool `hcl:"verbose"`
}

func (c Config) IsVerbose() bool {
	if c.Verbose == nil {
		return false
	}

	return *c.Verbose
}

func (c Config) Validate() error {
	if err := c.Compress.Validate(); err != nil {
		return err
	}
	return nil
}

func LoadConfig(location string) error {
	var cfg Config

	configLocation := "/etc/postgres_backup/config.hcl"
	if env, ok := os.LookupEnv("CONFIG_LOCATION"); ok {
		configLocation = env
	}

	if location != "" {
		configLocation = location
	}

	err := hclsimple.DecodeFile(configLocation, nil, &cfg)
	if err != nil {
		return err
	}

	Loaded = &cfg

	if err := cfg.Validate(); err != nil {
		return err
	}

	return nil
}
