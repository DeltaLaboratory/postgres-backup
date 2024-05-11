package internal

import (
	"errors"
	"io"

	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"

	"github.com/DeltaLaboratory/postgres-backup/internal/config"
)

func Compress(input io.Reader) (io.Reader, error) {
	r, w := io.Pipe()

	switch config.Loaded.Compress.Algorithm {
	case "zstd":
		encoder, err := zstd.NewWriter(w, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(*config.Loaded.Compress.CompressLevel)))
		if err != nil {
			return nil, err
		}

		go func() {
			encoder.ReadFrom(input)
			encoder.Close()
			w.Close()
		}()

		return r, nil
	case "gzip":
		writer, _ := gzip.NewWriterLevel(w, *config.Loaded.Compress.CompressLevel)

		go func() {
			io.Copy(writer, input)
			writer.Close()
			w.Close()
		}()

		return r, nil
	default:
		return nil, errors.New("unsupported compress algorithm")
	}
}
