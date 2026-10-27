package modelregistry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrChecksumMismatch means the downloaded bytes don't match registry.json's
// recorded sha256 — never write or serve a model the pointer file doesn't
// vouch for.
var ErrChecksumMismatch = errors.New("modelregistry: downloaded content sha256 does not match registry")

// Downloader fetches raw bytes for an R2 object key. The real
// implementation (r2.go) talks to Cloudflare R2; tests inject a fake.
type Downloader interface {
	Download(ctx context.Context, objectKey string) ([]byte, error)
}

// FetchModel returns a local, checksum-verified path to the model artifact
// reg describes. If cacheDir already has a file matching reg's sha256 (from
// a previous run — cacheDir is expected to be a persistent path on a
// long-lived CI runner, not a fresh tmpdir per run), it's returned without
// touching the network. Otherwise dl.Download is called, the result is
// verified against reg.SHA256, and — only if it matches — written into
// cacheDir before returning its path.
func FetchModel(ctx context.Context, reg Registry, cacheDir string, dl Downloader) (string, error) {
	cachedPath := filepath.Join(cacheDir, reg.ModelVersion+".json")

	if existing, err := os.ReadFile(cachedPath); err == nil {
		if sha256Hex(existing) == reg.SHA256 {
			return cachedPath, nil
		}
		// Stale/corrupted cache — fall through to re-download.
	}

	content, err := dl.Download(ctx, reg.R2ObjectKey)
	if err != nil {
		return "", fmt.Errorf("modelregistry: downloading %s: %w", reg.R2ObjectKey, err)
	}

	got := sha256Hex(content)
	if got != reg.SHA256 {
		return "", fmt.Errorf("%w: object %s got %s, registry says %s", ErrChecksumMismatch, reg.R2ObjectKey, got, reg.SHA256)
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", fmt.Errorf("modelregistry: creating cache dir %s: %w", cacheDir, err)
	}
	if err := os.WriteFile(cachedPath, content, 0o644); err != nil {
		return "", fmt.Errorf("modelregistry: writing %s: %w", cachedPath, err)
	}
	return cachedPath, nil
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
