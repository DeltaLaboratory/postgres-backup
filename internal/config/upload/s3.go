package upload

type S3Config struct {
	Endpoint  string `hcl:"endpoint"`
	AccessKey string `hcl:"access_key"`
	SecretKey string `hcl:"secret_key"`

	Bucket string  `hcl:"bucket"`
	Region *string `hcl:"region"`

	Prefix *string `hcl:"prefix"`
}

func (c S3Config) GetRegion() string {
	if c.Region == nil {
		return ""
	}
	return *c.Region
}
