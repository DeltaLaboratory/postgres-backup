package upload

type Upload struct {
	S3 *S3Config `hcl:"s3,block"`
}
