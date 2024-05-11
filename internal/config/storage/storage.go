package storage

type Storage struct {
	S3    *S3Storage    `hcl:"s3,block"`
	Local *LocalStorage `hcl:"local,block"`
}
