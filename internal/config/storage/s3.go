package storage

type S3Storage struct {
	Endpoint  string `hcl:"endpoint"`
	AccessKey string `hcl:"access_key"`
	SecretKey string `hcl:"secret_key"`

	Bucket string  `hcl:"bucket"`
	Region *string `hcl:"region"`

	Prefix *string `hcl:"prefix"`

	// Retention settings
	RetentionPeriod *string `hcl:"retention_period"`
	RetentionCount  *int    `hcl:"retention_count"`
}

func (s *S3Storage) GetRegion() string {
	if s.Region == nil {
		return ""
	}

	return *s.Region
}
