package config

type PostgresConfig struct {
	Version  int     `hcl:"version"`
	Host     string  `hcl:"host"`
	Port     *int    `hcl:"port"`
	User     *string `hcl:"user"`
	Password *string `hcl:"password"`
	Database *string `hcl:"database"`
}
