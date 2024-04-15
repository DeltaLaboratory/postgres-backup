# postgres-backup
[![Code Analysis](https://github.com/DeltaLaboratory/postgres-backup/actions/workflows/analysis.yml/badge.svg)](https://github.com/DeltaLaboratory/postgres-backup/actions/workflows/analysis.yml)
[![Build Container](https://github.com/DeltaLaboratory/postgres-backup/actions/workflows/container.yml/badge.svg)](https://github.com/DeltaLaboratory/postgres-backup/actions/workflows/container.yml)

Postgres-backup backup postgres database to local or remote storage.
# usage
## docker
```shell
docker run -v ./config.hcl:/etc/postgres_backup/config.hcl ghcr.io/deltalaboratory/postgres-backup:latest
```
## docker compose
```yaml
backup:
  image: ghcr.io/deltalaboratory/postgres-backup:v0.0.1
  volumes:
    - ./postgres-backup.hcl:/etc/postgres_backup/config.hcl
  restart: unless-stopped
```
# configuration
this project uses [HCL](https://github.com/hashicorp/hcl) for configuration file.
default configuration find path is "/etc/postgres_backup/config.hcl". this can be overridden by environment variable `CONFIG_PATH`.
## example configuration
```hcl
# postgres connection configuration
postgres {
  # postgres version
  version = "15"
  # postgres host
  host = "database"
  # postgres port (optional, default 5432)
  port = 5432
  # postgres user (optional)
  user = "postgres"
  # postgres password (optional)
  password = "postgres"
  # postgres database (optional, default postgres)
  database = "postgres"
}

# backup storage configuration
upload {
  # S3 storage configuration
  s3 {
    # S3 endpoint
    endpoint = "r2.cloudflarestorage.com"

    # S3 access key
    access_key = "33e7f63077b1c5bce4f1ecadd4d990cf229667c40bfb00686990c950911b7ab7"
    # S3 secret key
    secret_key = "33e7f63077b1c5bce4f1ecadd4d990cf229667c40bfb00686990c950911b7ab7"

    # S3 bucket
    bucket = "backup"
    # S3 region (optional)
    region = "auto"

    # S3 prefix (optional, backup file will be stored in `{prefix}/2006-01-02T15:04:05.{compress_algorithm}.sql`)
    prefix = "backup"
  }
}

# backup schedule, required when using `upload-schedule` command
# see https://pkg.go.dev/github.com/robfig/cron#hdr-CRON_Expression_Format for more information
schedule = [
  "0 1 * * *",
]

# compress algorithm, support `zstd`, optional
compress_algorithm = "zstd"
# compress level, optional
# for zstd, see https://github.com/klauspost/compress/tree/master/zstd#compressor for more information, default 3
compress_level = 12

# verbose mode
verbose = false
```

# TODO
- [ ] Add more storage support
- [ ] Add more compress algorithm
- [ ] Support multiple database backup
- [ ] Support notification
- [ ] Support backup retention
- [ ] Support backup restore
- [ ] Support streaming compress/upload backup
- [ ] Support backup encryption
- [ ] Support backup status dashboard?