package config

import (
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2/hclsimple"

	"github.com/DeltaLaboratory/postgres-backup/internal/config/storage"
)

var Loaded *Config

type RestoreScheduleConfig struct {
	Cron            string  `hcl:"cron"`
	TargetDatabase  string  `hcl:"target_database"`
	BackupSelection string  `hcl:"backup_selection"` // "latest", "pattern", "specific"
	BackupPattern   *string `hcl:"backup_pattern"`   // optional: for pattern-based selection
	BackupID        *string `hcl:"backup_id"`        // optional: for specific backup selection
	IncludeS3       *bool   `hcl:"include_s3"`
	IncludeLocal    *bool   `hcl:"include_local"`
	Enabled         *bool   `hcl:"enabled"`
}

func (r RestoreScheduleConfig) IsEnabled() bool {
	return r.Enabled == nil || *r.Enabled
}

func (r RestoreScheduleConfig) ShouldIncludeS3() bool {
	return r.IncludeS3 == nil || *r.IncludeS3
}

func (r RestoreScheduleConfig) ShouldIncludeLocal() bool {
	return r.IncludeLocal == nil || *r.IncludeLocal
}

type Config struct {
	Postgres        PostgresConfig          `hcl:"postgres,block"`
	Storage         storage.Storage         `hcl:"storage,block"`
	Compress        *CompressConfig         `hcl:"compress,block"`
	Schedule        []string                `hcl:"schedule"`
	RestoreSchedule []RestoreScheduleConfig `hcl:"restore_schedule,block"`
	Verbose         *bool                   `hcl:"verbose"`
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

	// Validate restore schedules
	for i, restoreSchedule := range c.RestoreSchedule {
		if err := c.validateRestoreSchedule(restoreSchedule, i); err != nil {
			return err
		}
	}

	return nil
}

func (c Config) validateRestoreSchedule(rs RestoreScheduleConfig, index int) error {
	// Validate required fields
	if rs.Cron == "" {
		return fmt.Errorf("restore_schedule[%d]: cron expression is required", index)
	}

	if rs.TargetDatabase == "" {
		return fmt.Errorf("restore_schedule[%d]: target_database is required", index)
	}

	if rs.BackupSelection == "" {
		return fmt.Errorf("restore_schedule[%d]: backup_selection is required", index)
	}

	// Validate backup_selection values
	validSelections := []string{"latest", "pattern", "specific"}
	validSelection := false
	for _, valid := range validSelections {
		if rs.BackupSelection == valid {
			validSelection = true
			break
		}
	}
	if !validSelection {
		return fmt.Errorf("restore_schedule[%d]: backup_selection must be one of %v, got '%s'",
			index, validSelections, rs.BackupSelection)
	}

	// Validate selection-specific requirements
	switch rs.BackupSelection {
	case "pattern":
		if rs.BackupPattern == nil || *rs.BackupPattern == "" {
			return fmt.Errorf("restore_schedule[%d]: backup_pattern is required when backup_selection is 'pattern'", index)
		}
	case "specific":
		if rs.BackupID == nil || *rs.BackupID == "" {
			return fmt.Errorf("restore_schedule[%d]: backup_id is required when backup_selection is 'specific'", index)
		}
	}

	// Validate that at least one storage source is enabled
	if !rs.ShouldIncludeS3() && !rs.ShouldIncludeLocal() {
		return fmt.Errorf("restore_schedule[%d]: at least one of include_s3 or include_local must be true", index)
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
