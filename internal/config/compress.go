package config

import (
	"fmt"
	"slices"

	"github.com/klauspost/compress/gzip"
)

var zstdDefaultCompressLevel = 3
var gzipDefaultCompressLevel = gzip.DefaultCompression

var compressAlgorithm = []string{
	"zstd",
	"gzip",
}

type CompressConfig struct {
	Algorithm     string `hcl:"algorithm"`
	CompressLevel *int   `hcl:"compress_level"`
}

func (c *CompressConfig) Validate() error {
	if !slices.Contains(compressAlgorithm, c.Algorithm) {
		return fmt.Errorf("compress.algorithm: unsupported algorithm")
	}

	if c.CompressLevel == nil {
		switch c.Algorithm {
		case "zstd":
			c.CompressLevel = &zstdDefaultCompressLevel
		case "gzip":
			c.CompressLevel = &gzipDefaultCompressLevel
		default:
			return fmt.Errorf("compress.compress_level: no default level specified for algorithm %s", c.Algorithm)
		}
	}

	return nil
}
