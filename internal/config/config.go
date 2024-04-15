package config

import (
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2/hclsimple"

	"postgres-backup/internal/config/upload"
)

var Loaded *Config

type Config struct {
	Postgres PostgresConfig `hcl:"postgres,block"`
	Upload   upload.Upload  `hcl:"upload,block"`

	Schedule []string `hcl:"schedule"`

	CompressAlgorithm *string `hcl:"compress_algorithm"`
	CompressLevel     *int    `hcl:"compress_level"`

	Verbose *bool `hcl:"verbose"`
}

func (c Config) IsVerbose() bool {
	if c.Verbose == nil {
		return false
	}

	return *c.Verbose
}

func (c Config) Validate() error {
	if c.CompressAlgorithm != nil {
		if *c.CompressAlgorithm != "zstd" {
			return fmt.Errorf("invalid compress algorithm: %s", *c.CompressAlgorithm)
		}

		if c.CompressLevel == nil {
			if *c.CompressAlgorithm == "zstd" {
				c.CompressLevel = new(int)
				*c.CompressLevel = 3
			}
		}
	}
	return nil
}

func LoadConfig() error {
	var cfg Config

	configLocation := "/etc/postgres_backup/config.hcl"
	if env, ok := os.LookupEnv("CONFIG_LOCATION"); ok {
		configLocation = env
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
