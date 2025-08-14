package config

import (
	"fmt"
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

	// Validate storage retention settings
	if c.Storage.S3 != nil {
		// Validate retention_period
		if c.Storage.S3.RetentionPeriod != nil {
			if _, err := c.Storage.S3.GetEffectiveRetentionDays(); err != nil {
				return fmt.Errorf("s3 retention_period validation failed: %w", err)
			}
		}

		// Validate retention_count
		if c.Storage.S3.RetentionCount != nil && *c.Storage.S3.RetentionCount <= 0 {
			return fmt.Errorf("s3 retention_count must be positive, got %d", *c.Storage.S3.RetentionCount)
		}
	}

	if c.Storage.Local != nil {
		// Validate retention_period
		if c.Storage.Local.RetentionPeriod != nil {
			if _, err := c.Storage.Local.GetEffectiveRetentionDays(); err != nil {
				return fmt.Errorf("local retention_period validation failed: %w", err)
			}
		}

		// Validate retention_count
		if c.Storage.Local.RetentionCount != nil && *c.Storage.Local.RetentionCount <= 0 {
			return fmt.Errorf("local retention_count must be positive, got %d", *c.Storage.Local.RetentionCount)
		}
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
