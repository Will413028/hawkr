//go:build integration

package storage

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	garageImage  = "dxflrs/garage:v2.5.0@sha256:677e5b758e4425a325b811c425c026bf35a4bf42f7ddbaa773de3f4fd779a5a5"
	garageBucket = "media-test"
	garageRegion = "garage"
	garageKey    = "media-test-key"
)

const garageConfig = `metadata_dir = "/tmp/meta"
data_dir = "/tmp/data"
db_engine = "sqlite"
replication_factor = 1
rpc_bind_addr = "127.0.0.1:3901"
rpc_secret = "0000000000000000000000000000000000000000000000000000000000000001"

[s3_api]
s3_region = "garage"
api_bind_addr = "0.0.0.0:3900"
`

var (
	garageKeyIDRe  = regexp.MustCompile(`Key ID:\s*(GK[0-9a-f]+)`)
	garageSecretRe = regexp.MustCompile(`Secret key:\s*([0-9a-f]{64})`)
	garageNodeRe   = regexp.MustCompile(`[0-9a-f]{64}`)
)

// garageExec 在容器內執行 garage CLI 並回傳合併輸出；結束碼非 0 視為錯誤。
func garageExec(ctx context.Context, c testcontainers.Container, args ...string) (string, error) {
	code, r, err := c.Exec(ctx, append([]string{"/garage"}, args...), tcexec.Multiplexed())
	if err != nil {
		return "", err
	}
	out, _ := io.ReadAll(r)
	if code != 0 {
		return string(out), fmt.Errorf("garage %s: exit %d", strings.Join(args, " "), code)
	}
	return string(out), nil
}

// retryGarage 重試到成功：節點剛啟動時 RPC 與 layout 還沒就緒。
func retryGarage(ctx context.Context, t *testing.T, c testcontainers.Container, args ...string) string {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for {
		out, err := garageExec(ctx, c, args...)
		if err == nil {
			return out
		}
		if time.Now().After(deadline) {
			t.Fatalf("%v\n%s", err, out)
		}
		time.Sleep(time.Second)
	}
}

// startGarage 起單節點 Garage、初始化 layout、key 與 bucket，回傳經 s3compat 設定建構的 adapter 與 endpoint。
func startGarage(t *testing.T) (*S3, string) {
	t.Helper()
	ctx := context.Background()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        garageImage,
			ExposedPorts: []string{"3900/tcp"},
			Files: []testcontainers.ContainerFile{{
				Reader:            strings.NewReader(garageConfig),
				ContainerFilePath: "/etc/garage.toml",
				FileMode:          0o644,
			}},
			WaitingFor: wait.ForListeningPort("3900/tcp").WithStartupTimeout(90 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	testcontainers.CleanupContainer(t, c)

	nodeOut := retryGarage(ctx, t, c, "node", "id", "-q")
	node := garageNodeRe.FindString(nodeOut)
	if node == "" {
		t.Fatalf("node id not found in output: %q", nodeOut)
	}
	retryGarage(ctx, t, c, "layout", "assign", "-z", "dc1", "-c", "1G", node)
	retryGarage(ctx, t, c, "layout", "apply", "--version", "1")

	keyOut := retryGarage(ctx, t, c, "key", "create", garageKey)
	id := garageKeyIDRe.FindStringSubmatch(keyOut)
	if id == nil {
		t.Fatalf("key id not found in key create output")
	}
	secret := garageSecretRe.FindStringSubmatch(keyOut)
	if secret == nil {
		infoOut := retryGarage(ctx, t, c, "key", "info", "--show-secret", garageKey)
		secret = garageSecretRe.FindStringSubmatch(infoOut)
	}
	if secret == nil {
		t.Fatal("secret key not found in garage output")
	}
	retryGarage(ctx, t, c, "bucket", "create", garageBucket)
	retryGarage(ctx, t, c, "bucket", "allow", "--read", "--write", "--owner", garageBucket, "--key", garageKey)

	endpoint, err := c.PortEndpoint(ctx, "3900/tcp", "http")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := envFrom(map[string]string{
		"MEDIA_STORAGE":              "s3compat",
		"MEDIA_S3_ENDPOINT":          endpoint,
		"MEDIA_S3_REGION":            garageRegion,
		"MEDIA_S3_BUCKET":            garageBucket,
		"MEDIA_S3_ACCESS_KEY_ID":     id[1],
		"MEDIA_S3_SECRET_ACCESS_KEY": secret[1],
	})
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return s, endpoint
}

func md5B64(b []byte) string {
	sum := md5.Sum(b)
	return base64.StdEncoding.EncodeToString(sum[:])
}

// sendPresigned 以純 net/http 送出預簽 PUT：header 取自簽發結果，可用 override 覆蓋個別 header；
// Content-Length 由 body 長度決定（Go 不允許用 header 設定）。回傳 HTTP 狀態碼。
func sendPresigned(t *testing.T, p PresignedPut, body []byte, override map[string]string) int {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPut, p.URL, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range p.Headers {
		if k == "Content-Length" {
			continue
		}
		req.Header.Set(k, v)
	}
	for k, v := range override {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send presigned put: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

func presign(t *testing.T, s *S3, key string, body []byte, contentType, md5 string) PresignedPut {
	t.Helper()
	p, err := s.PresignPut(context.Background(), PresignPutInput{
		Key: key, ContentLength: int64(len(body)), ContentType: contentType, ContentMD5: md5, Expires: 5 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func readAll(t *testing.T, s *S3, key string) []byte {
	t.Helper()
	r, err := s.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("get %s: %v", key, err)
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func requireAbsent(t *testing.T, s *S3, key string) {
	t.Helper()
	if _, err := s.Head(context.Background(), key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("head %s: want ErrNotFound, got %v", key, err)
	}
}

func requireRejected(t *testing.T, status int) {
	t.Helper()
	if status < 400 || status >= 500 {
		t.Fatalf("want 4xx rejection, got HTTP %d", status)
	}
}

func TestGarageStore(t *testing.T) {
	s, _ := startGarage(t)
	ctx := context.Background()

	t.Run("presigned_put_as_signed", func(t *testing.T) {
		key := NewKey(PrefixTmp)
		body := []byte("hello presigned world")
		p := presign(t, s, key, body, "text/plain", md5B64(body))
		if st := sendPresigned(t, p, body, nil); st != http.StatusOK {
			t.Fatalf("want 200, got %d", st)
		}
		if got := readAll(t, s, key); !bytes.Equal(got, body) {
			t.Fatalf("got %q want %q", got, body)
		}
		info, err := s.Head(ctx, key)
		if err != nil {
			t.Fatal(err)
		}
		if info.Size != int64(len(body)) || info.ContentType != "text/plain" {
			t.Fatalf("head = %+v", info)
		}
	})

	t.Run("size_mismatch", func(t *testing.T) {
		key := NewKey(PrefixTmp)
		signed := []byte("signed-size")
		actual := []byte("a body that is longer than the signed size")
		// Content-MD5 對應實際送出的內容，只有長度與簽章不符。
		p := presign(t, s, key, signed, "text/plain", md5B64(actual))
		requireRejected(t, sendPresigned(t, p, actual, nil))
		requireAbsent(t, s, key)
	})

	t.Run("content_type_mismatch", func(t *testing.T) {
		key := NewKey(PrefixTmp)
		body := []byte("<html>not an image</html>")
		p := presign(t, s, key, body, "image/png", md5B64(body))
		requireRejected(t, sendPresigned(t, p, body, map[string]string{"Content-Type": "text/html"}))
		requireAbsent(t, s, key)
	})

	t.Run("existing_key", func(t *testing.T) {
		key := NewKey(PrefixTmp)
		first := []byte("first writer wins")
		second := []byte("second writer loses")
		if st := sendPresigned(t, presign(t, s, key, first, "text/plain", md5B64(first)), first, nil); st != http.StatusOK {
			t.Fatalf("first put: want 200, got %d", st)
		}
		requireRejected(t, sendPresigned(t, presign(t, s, key, second, "text/plain", md5B64(second)), second, nil))
		if got := readAll(t, s, key); !bytes.Equal(got, first) {
			t.Fatalf("object changed: got %q want %q", got, first)
		}
	})

	t.Run("wrong_content_md5", func(t *testing.T) {
		key := NewKey(PrefixTmp)
		body := []byte("content whose digest is lied about")
		p := presign(t, s, key, body, "text/plain", md5B64([]byte("something else")))
		requireRejected(t, sendPresigned(t, p, body, nil))
		requireAbsent(t, s, key)
	})

	t.Run("round_trip", func(t *testing.T) {
		key := NewKey(PrefixPublic)
		body := []byte("round trip bytes")
		if err := s.Put(ctx, PutInput{Key: key, Body: bytes.NewReader(body), Size: int64(len(body)), ContentType: "application/x-test"}); err != nil {
			t.Fatal(err)
		}
		info, err := s.Head(ctx, key)
		if err != nil {
			t.Fatal(err)
		}
		if info.Size != int64(len(body)) || info.ContentType != "application/x-test" {
			t.Fatalf("head = %+v", info)
		}
		if got := readAll(t, s, key); !bytes.Equal(got, body) {
			t.Fatalf("got %q want %q", got, body)
		}
		if err := s.Delete(ctx, key); err != nil {
			t.Fatal(err)
		}
		requireAbsent(t, s, key)
		if _, err := s.Get(ctx, key); !errors.Is(err, ErrNotFound) {
			t.Fatalf("get after delete: want ErrNotFound, got %v", err)
		}
		if err := s.Delete(ctx, key); err != nil {
			t.Fatalf("delete of missing key: %v", err)
		}
	})
}
