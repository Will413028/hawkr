//go:build integration

package storage

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // Content-MD5 是 S3 協定要求的完整性 header，不作安全用途
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

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
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	code, r, err := c.Exec(ctx, append([]string{"/garage"}, args...), tcexec.Multiplexed())
	if err != nil {
		return "", err
	}
	out, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("garage %s: read output: %w", strings.Join(args, " "), err)
	}
	if code != 0 {
		return string(out), fmt.Errorf("garage %s: exit %d", strings.Join(args, " "), code)
	}
	return string(out), nil
}

// waitGarageReady 只重試唯讀的就緒探測：節點剛啟動時 RPC 還沒就緒；之後的每個變更指令只執行一次。
func waitGarageReady(ctx context.Context, t *testing.T, c testcontainers.Container) string {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for {
		out, err := garageExec(ctx, c, "node", "id", "-q")
		if err == nil {
			return out
		}
		if time.Now().After(deadline) {
			t.Fatalf("%v\n%s", err, out)
		}
		time.Sleep(time.Second)
	}
}

// mustGarage 執行一次 garage 指令，失敗即中止。
func mustGarage(ctx context.Context, t *testing.T, c testcontainers.Container, args ...string) string {
	t.Helper()
	out, err := garageExec(ctx, c, args...)
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	return out
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

	nodeOut := waitGarageReady(ctx, t, c)
	node := garageNodeRe.FindString(nodeOut)
	if node == "" {
		t.Fatalf("node id not found in output: %q", nodeOut)
	}
	mustGarage(ctx, t, c, "layout", "assign", "-z", "dc1", "-c", "1G", node)
	mustGarage(ctx, t, c, "layout", "apply", "--version", "1")

	keyOut := mustGarage(ctx, t, c, "key", "create", garageKey)
	id := garageKeyIDRe.FindStringSubmatch(keyOut)
	if id == nil {
		t.Fatalf("key id not found in key create output")
	}
	secret := garageSecretRe.FindStringSubmatch(keyOut)
	if secret == nil {
		infoOut := mustGarage(ctx, t, c, "key", "info", "--show-secret", garageKey)
		secret = garageSecretRe.FindStringSubmatch(infoOut)
	}
	if secret == nil {
		t.Fatal("secret key not found in garage output")
	}
	mustGarage(ctx, t, c, "bucket", "create", garageBucket)
	mustGarage(ctx, t, c, "bucket", "allow", "--read", "--write", "--owner", garageBucket, "--key", garageKey)

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
	sum := md5.Sum(b) //nolint:gosec // 同上
	return base64.StdEncoding.EncodeToString(sum[:])
}

// sendPresigned 以純 net/http 送出預簽 PUT：header 取自簽發結果，可用 override 覆蓋個別 header；
// 簽發結果不含 Content-Length，它由 body 長度決定。回傳 HTTP 狀態碼。
// override 的值為空字串時刪除該 header。
func sendPresigned(t *testing.T, p PresignedPut, body []byte, override map[string]string) presignedResp {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPut, p.URL, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range p.Headers {
		req.Header.Set(k, v)
	}
	for k, v := range override {
		if v == "" {
			req.Header.Del(k)
			continue
		}
		req.Header.Set(k, v)
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("send presigned put: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	_, _ = io.Copy(io.Discard, resp.Body)
	return presignedResp{Status: resp.StatusCode, Body: string(b)}
}

type presignedResp struct {
	Status int
	Body   string
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
	defer func() { _ = r.Close() }()
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

func requireStatus(t *testing.T, got presignedResp, want int) {
	t.Helper()
	if got.Status != want {
		t.Fatalf("want HTTP %d, got %d: %s", want, got.Status, got.Body)
	}
}

func TestGarageStore(t *testing.T) {
	s, _ := startGarage(t)
	ctx := context.Background()

	t.Run("presigned_put_as_signed", func(t *testing.T) {
		key := NewKey(PrefixTmp)
		body := []byte("hello presigned world")
		p := presign(t, s, key, body, "text/plain", md5B64(body))
		if st := sendPresigned(t, p, body, nil); st.Status != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", st.Status, st.Body)
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
		requireStatus(t, sendPresigned(t, p, actual, nil), http.StatusForbidden)
		requireAbsent(t, s, key)
	})

	t.Run("content_type_mismatch", func(t *testing.T) {
		key := NewKey(PrefixTmp)
		body := []byte("<html>not an image</html>")
		p := presign(t, s, key, body, "image/png", md5B64(body))
		requireStatus(t, sendPresigned(t, p, body, map[string]string{"Content-Type": "text/html"}), http.StatusForbidden)
		requireAbsent(t, s, key)
	})

	t.Run("md5_binding_override_rejected", func(t *testing.T) {
		key := NewKey(PrefixTmp)
		signedBody := []byte("signed body AAAA")
		other := []byte("other  body BBBB")
		p := presign(t, s, key, signedBody, "text/plain", md5B64(signedBody))
		requireStatus(t, sendPresigned(t, p, other, map[string]string{"Content-MD5": md5B64(other)}), http.StatusForbidden)
		requireAbsent(t, s, key)
	})

	t.Run("md5_binding_removed_rejected", func(t *testing.T) {
		key := NewKey(PrefixTmp)
		signedBody := []byte("signed body AAAA")
		other := []byte("other  body BBBB")
		p := presign(t, s, key, signedBody, "text/plain", md5B64(signedBody))
		requireStatus(t, sendPresigned(t, p, other, map[string]string{"Content-MD5": ""}), http.StatusBadRequest)
		requireAbsent(t, s, key)
	})

	t.Run("digest_check_wrong_content_md5", func(t *testing.T) {
		key := NewKey(PrefixTmp)
		body := []byte("content whose digest is lied about")
		p := presign(t, s, key, body, "text/plain", md5B64([]byte("something else")))
		requireStatus(t, sendPresigned(t, p, body, nil), http.StatusBadRequest)
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

	// Garage 支援依前綴列出：機敏資產的 GC 靠它找出沒推進成功的孤兒嘗試（B1-3c1 的停止條件）。
	t.Run("list_by_prefix", func(t *testing.T) {
		asset := uuid.New()
		other := uuid.New()
		want := []string{NewSecureChunkKey(asset, 0), NewSecureChunkKey(asset, 0), NewSecureChunkKey(asset, 1), NewSecureChunkKey(asset, 10)}
		for _, k := range append(slices.Clone(want), NewSecureChunkKey(other, 0)) {
			if err := s.Put(ctx, PutInput{Key: k, Body: bytes.NewReader([]byte("x")), Size: 1}); err != nil {
				t.Fatal(err)
			}
		}
		got, err := s.List(ctx, SecurePrefix(asset))
		if err != nil {
			t.Fatal(err)
		}
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Fatalf("List = %v，預期 %v", got, want)
		}
		none, err := s.List(ctx, SecurePrefix(uuid.New()))
		if err != nil || len(none) != 0 {
			t.Fatalf("沒有符合的前綴：%v %v", none, err)
		}
	})
}
