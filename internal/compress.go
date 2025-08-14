package internal

import (
	"errors"
	"io"
	"strings"

	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"
	"github.com/rs/zerolog/log"

	"github.com/DeltaLaboratory/postgres-backup/internal/config"
)

const (
	algorithmZstd = "zstd"
	algorithmGzip = "gzip"
)

func Compress(input io.Reader) (io.Reader, error) {
	r, w := io.Pipe()

	switch config.Loaded.Compress.Algorithm {
	case algorithmZstd:
		encoder, err := zstd.NewWriter(w, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(*config.Loaded.Compress.CompressLevel)))
		if err != nil {
			return nil, err
		}

		go func() {
			if _, err := encoder.ReadFrom(input); err != nil {
				log.Error().Err(err).Msg("failed to read from input during zstd compression")
			}
			encoder.Close()
			w.Close()
		}()

		return r, nil
	case algorithmGzip:
		writer, _ := gzip.NewWriterLevel(w, *config.Loaded.Compress.CompressLevel)

		go func() {
			if _, err := io.Copy(writer, input); err != nil {
				log.Error().Err(err).Msg("failed to copy input during gzip compression")
			}
			writer.Close()
			w.Close()
		}()

		return r, nil
	default:
		return nil, errors.New("unsupported compress algorithm")
	}
}

// Decompress decompresses the input stream based on the detected compression algorithm
func Decompress(input io.Reader, filename string) (io.Reader, error) {
	// Detect compression algorithm from filename
	var algorithm string
	switch {
	case strings.HasSuffix(filename, ".zstd"):
		algorithm = algorithmZstd
	case strings.HasSuffix(filename, ".gzip"):
		algorithm = algorithmGzip
	default:
		// No compression detected, return input as-is
		return input, nil
	}

	switch algorithm {
	case algorithmZstd:
		decoder, err := zstd.NewReader(input)
		if err != nil {
			return nil, err
		}
		return decoder, nil
	case algorithmGzip:
		reader, err := gzip.NewReader(input)
		if err != nil {
			return nil, err
		}
		return reader, nil
	default:
		return input, nil
	}
}
