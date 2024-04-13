package config

import (
	"os"

	"github.com/hashicorp/hcl/v2/hclsimple"
)

var Loaded *Config

type Config struct {
	Postgres PostgresConfig `hcl:"postgres,block"`

	Verbose *bool `hcl:"verbose"`
}

func (c Config) IsVerbose() bool {
	if c.Verbose == nil {
		return false
	}

	return *c.Verbose
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

	return nil
}
