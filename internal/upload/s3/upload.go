package s3

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/rs/zerolog/log"

	"postgres-backup/internal/config"
)

func Upload(ctx context.Context, data []byte) error {
	logger := log.Logger.With().Str("caller", "upload_s3").Logger()

	if config.Loaded.Upload.S3 == nil {
		return fmt.Errorf("s3: config is not present")
	}

	client, err := minio.New(config.Loaded.Upload.S3.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.Loaded.Upload.S3.AccessKey, config.Loaded.Upload.S3.SecretKey, ""),
		Region: config.Loaded.Upload.S3.GetRegion(),
		Secure: true,
	})

	if err != nil {
		return fmt.Errorf("s3: failed to create client: %w", err)
	}

	logger.Info().Msg("uploading dump to s3")

	objectName := fmt.Sprintf("%s", time.Now().Format("2006-01-02T15:04:05"))

	if config.Loaded.Upload.S3.Prefix != nil {
		objectName = fmt.Sprintf("%s/%s", *config.Loaded.Upload.S3.Prefix, objectName)
	}

	if config.Loaded.CompressAlgorithm != nil {
		objectName = fmt.Sprintf("%s.%s", objectName, *config.Loaded.CompressAlgorithm)
	}

	objectName = fmt.Sprintf("%s.%s", objectName, "sql")

	info, err := client.PutObject(ctx, config.Loaded.Upload.S3.Bucket, objectName, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("s3: failed to upload dump: %w", err)
	}

	logger.Info().Str("key", info.Key).Msg("dump uploaded")
	return nil
}
