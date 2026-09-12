package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/surajkadam7/iot-backend/internal/config"
)

// Open picks a Store from config. Empty S3_BUCKET uses the local filesystem
// so developers can archive/export without AWS. A later Firehose path would
// still read the same key layout through this interface.
func Open(ctx context.Context, cfg config.Config) (Store, error) {
	if !cfg.ArchiveEnabled {
		return nil, nil
	}
	backend := strings.ToLower(strings.TrimSpace(cfg.ArchiveBackend))
	if backend == "" || backend == "auto" {
		if strings.TrimSpace(cfg.S3Bucket) != "" {
			backend = "s3"
		} else {
			backend = "fs"
		}
	}
	switch backend {
	case "fs", "filesystem", "file":
		return NewFilesystem(cfg.ArchiveDir)
	case "s3":
		return NewS3(ctx, cfg)
	case "memory":
		return NewMemory(), nil
	default:
		return nil, fmt.Errorf("unknown ARCHIVE_BACKEND %q (use auto, fs, s3)", backend)
	}
}
