package compress

import (
	"strings"

	"github.com/klauspost/compress/zstd"

	"postgres-backup/internal/config"
)

func Compress(input []byte) ([]byte, error) {
	if config.Loaded.CompressAlgorithm == nil {
		return input, nil
	}
	switch strings.ToLower(*config.Loaded.CompressAlgorithm) {
	case "zstd":
		encoder, _ := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(*config.Loaded.CompressLevel)))
		return encoder.EncodeAll(input, make([]byte, 0, len(input))), nil
	}
	return input, nil
}
