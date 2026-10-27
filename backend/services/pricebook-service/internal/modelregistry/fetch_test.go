package modelregistry

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type fakeDownloader struct {
	calls   int
	content []byte
	err     error
}

func (f *fakeDownloader) Download(_ context.Context, _ string) ([]byte, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.content, nil
}

func TestFetchModel_DownloadsAndVerifiesOnCacheMiss(t *testing.T) {
	content := []byte(`{"fake":"model"}`)
	reg := Registry{ModelVersion: "v1", R2ObjectKey: "key1", SHA256: sha256Hex(content)}
	dl := &fakeDownloader{content: content}
	cacheDir := t.TempDir()

	path, err := FetchModel(context.Background(), reg, cacheDir, dl)
	if err != nil {
		t.Fatalf("FetchModel: %v", err)
	}
	if dl.calls != 1 {
		t.Errorf("Download called %d times, want 1", dl.calls)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading result path %s: %v", path, err)
	}
	if string(got) != string(content) {
		t.Errorf("cached content = %q, want %q", got, content)
	}
}

func TestFetchModel_SkipsDownloadWhenCacheMatches(t *testing.T) {
	content := []byte(`{"fake":"model"}`)
	reg := Registry{ModelVersion: "v1", R2ObjectKey: "key1", SHA256: sha256Hex(content)}
	cacheDir := t.TempDir()
	// Pre-populate the cache with the file FetchModel would itself write,
	// at the same path it would compute.
	cachedPath := filepath.Join(cacheDir, reg.ModelVersion+".json")
	if err := os.WriteFile(cachedPath, content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	dl := &fakeDownloader{content: content}

	path, err := FetchModel(context.Background(), reg, cacheDir, dl)
	if err != nil {
		t.Fatalf("FetchModel: %v", err)
	}
	if dl.calls != 0 {
		t.Errorf("Download called %d times, want 0 (cache hit should skip network)", dl.calls)
	}
	if path != cachedPath {
		t.Errorf("path = %q, want %q", path, cachedPath)
	}
}

func TestFetchModel_RedownloadsWhenCacheChecksumStale(t *testing.T) {
	content := []byte(`{"fake":"model"}`)
	reg := Registry{ModelVersion: "v1", R2ObjectKey: "key1", SHA256: sha256Hex(content)}
	cacheDir := t.TempDir()
	cachedPath := filepath.Join(cacheDir, reg.ModelVersion+".json")
	// Stale/corrupted cache: different content than what SHA256 promises.
	if err := os.WriteFile(cachedPath, []byte("stale garbage"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	dl := &fakeDownloader{content: content}

	_, err := FetchModel(context.Background(), reg, cacheDir, dl)
	if err != nil {
		t.Fatalf("FetchModel: %v", err)
	}
	if dl.calls != 1 {
		t.Errorf("Download called %d times, want 1 (stale cache should trigger re-download)", dl.calls)
	}
}

func TestFetchModel_ChecksumMismatchAfterDownloadFails(t *testing.T) {
	reg := Registry{ModelVersion: "v1", R2ObjectKey: "key1", SHA256: "deadbeef"}
	dl := &fakeDownloader{content: []byte("wrong content")}
	cacheDir := t.TempDir()

	_, err := FetchModel(context.Background(), reg, cacheDir, dl)
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Errorf("err = %v, want ErrChecksumMismatch", err)
	}
}

func TestFetchModel_DownloadError(t *testing.T) {
	reg := Registry{ModelVersion: "v1", R2ObjectKey: "key1", SHA256: "abc"}
	wantErr := errors.New("network boom")
	dl := &fakeDownloader{err: wantErr}
	cacheDir := t.TempDir()

	_, err := FetchModel(context.Background(), reg, cacheDir, dl)
	if !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want wrapping %v", err, wantErr)
	}
}
