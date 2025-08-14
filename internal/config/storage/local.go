package storage

type LocalStorage struct {
	Directory string `hcl:"directory"`

	// Retention settings
	RetentionPeriod *string `hcl:"retention_period"`
	RetentionCount  *int    `hcl:"retention_count"`
}
