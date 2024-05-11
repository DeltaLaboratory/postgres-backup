package s3

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/rs/zerolog/log"

	"github.com/DeltaLaboratory/postgres-backup/internal/config"
)

func Upload(ctx context.Context, reader io.Reader) error {
	logger := log.Logger.With().Str("caller", "upload_s3").Logger()

	if config.Loaded.Storage.S3 == nil {
		return fmt.Errorf("s3: config is not present")
	}

	client, err := minio.New(config.Loaded.Storage.S3.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.Loaded.Storage.S3.AccessKey, config.Loaded.Storage.S3.SecretKey, ""),
		Region: config.Loaded.Storage.S3.GetRegion(),
		Secure: true,
	})

	if err != nil {
		return fmt.Errorf("s3: failed to create client: %w", err)
	}

	if config.Loaded.IsVerbose() {
		client.TraceOn(log.Logger)
	}

	logger.Info().Msg("uploading dump to s3")

	objectName := time.Now().Format("2006-01-02T15:04:05")

	if config.Loaded.Storage.S3.Prefix != nil {
		objectName = fmt.Sprintf("%s/%s", *config.Loaded.Storage.S3.Prefix, objectName)
	}

	if config.Loaded.Compress != nil {
		objectName = fmt.Sprintf("%s.%s", objectName, config.Loaded.Compress.Algorithm)
	}

	objectName = fmt.Sprintf("%s.sql", objectName)

	info, err := client.PutObject(ctx, config.Loaded.Storage.S3.Bucket, objectName, reader, -1, minio.PutObjectOptions{
		SendContentMd5: true,
	})
	if err != nil {
		return fmt.Errorf("s3: failed to storage dump: %w", err)
	}

	logger.Info().Str("key", info.Key).Msg("dump uploaded")
	return nil
}
