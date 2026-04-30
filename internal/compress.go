package internal

import (
	"errors"
	"io"
	"strings"

	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"

	"github.com/DeltaLaboratory/postgres-backup/internal/config"
)

const (
	algorithmZstd = "zstd"
	algorithmGzip = "gzip"
)

func Compress(input io.Reader) (io.ReadCloser, error) {
	r, w := io.Pipe()

	switch config.Loaded.Compress.Algorithm {
	case algorithmZstd:
		encoder, err := zstd.NewWriter(w, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(*config.Loaded.Compress.CompressLevel)))
		if err != nil {
			return nil, err
		}

		go func() {
			_, copyErr := encoder.ReadFrom(input)
			closeErr := encoder.Close()
			switch {
			case copyErr != nil:
				w.CloseWithError(copyErr)
			case closeErr != nil:
				w.CloseWithError(closeErr)
			default:
				w.Close()
			}
		}()

		return r, nil
	case algorithmGzip:
		writer, err := gzip.NewWriterLevel(w, *config.Loaded.Compress.CompressLevel)
		if err != nil {
			return nil, err
		}

		go func() {
			_, copyErr := io.Copy(writer, input)
			closeErr := writer.Close()
			switch {
			case copyErr != nil:
				w.CloseWithError(copyErr)
			case closeErr != nil:
				w.CloseWithError(closeErr)
			default:
				w.Close()
			}
		}()

		return r, nil
	default:
		return nil, errors.New("unsupported compress algorithm")
	}
}

// Decompress decompresses the input stream based on the detected compression algorithm
func Decompress(input io.Reader, filename string) (io.ReadCloser, error) {
	// Detect compression algorithm from filename
	var algorithm string
	switch {
	case strings.HasSuffix(filename, ".zstd"):
		algorithm = algorithmZstd
	case strings.HasSuffix(filename, ".gzip"):
		algorithm = algorithmGzip
	default:
		// No compression detected, return input as-is
		return io.NopCloser(input), nil
	}

	switch algorithm {
	case algorithmZstd:
		decoder, err := zstd.NewReader(input)
		if err != nil {
			return nil, err
		}
		return decoder.IOReadCloser(), nil
	case algorithmGzip:
		reader, err := gzip.NewReader(input)
		if err != nil {
			return nil, err
		}
		return reader, nil
	default:
		return io.NopCloser(input), nil
	}
}
