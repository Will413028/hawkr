package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/config"
)

var uuidV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func envFrom(m map[string]string) (Config, error) {
	e := config.FromMap(m)
	c := ConfigFromEnv(e)
	return c, e.Err()
}

func r2Env() map[string]string {
	return map[string]string{
		"MEDIA_STORAGE":              "r2",
		"MEDIA_R2_ACCOUNT_ID":        "acct123",
		"MEDIA_R2_BUCKET":            "bkt",
		"MEDIA_R2_ACCESS_KEY_ID":     "AKID",
		"MEDIA_R2_SECRET_ACCESS_KEY": "SECRET",
	}
}

func s3Env() map[string]string {
	return map[string]string{
		"MEDIA_STORAGE":              "s3compat",
		"MEDIA_S3_ENDPOINT":          "http://localhost:9000",
		"MEDIA_S3_REGION":            "us-east-1",
		"MEDIA_S3_BUCKET":            "bkt",
		"MEDIA_S3_ACCESS_KEY_ID":     "AKID",
		"MEDIA_S3_SECRET_ACCESS_KEY": "SECRET",
	}
}

func TestConfigRejectsMissingOrInvalidSelection(t *testing.T) {
	for name, m := range map[string]map[string]string{
		"unset":   {},
		"empty":   {"MEDIA_STORAGE": ""},
		"unknown": {"MEDIA_STORAGE": "fake"},
		"upper":   {"MEDIA_STORAGE": "R2"},
	} {
		if _, err := envFrom(m); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestConfigRejectsEverySingleMissingSetting(t *testing.T) {
	for _, base := range []map[string]string{r2Env(), s3Env()} {
		for k := range base {
			m := map[string]string{}
			for kk, v := range base {
				m[kk] = v
			}
			delete(m, k)
			if _, err := envFrom(m); err == nil {
				t.Errorf("%s: missing %s accepted", base["MEDIA_STORAGE"], k)
			}
			m[k] = ""
			if _, err := envFrom(m); err == nil {
				t.Errorf("%s: empty %s accepted", base["MEDIA_STORAGE"], k)
			}
		}
	}
}

func TestConfigR2AndS3Compat(t *testing.T) {
	c, err := envFrom(r2Env())
	if err != nil {
		t.Fatal(err)
	}
	if c.Endpoint != "https://acct123.r2.cloudflarestorage.com" || c.Region != "auto" || c.PathStyle {
		t.Fatalf("r2 config: %+v", c)
	}
	c, err = envFrom(s3Env())
	if err != nil {
		t.Fatal(err)
	}
	if c.Endpoint != "http://localhost:9000" || !c.PathStyle {
		t.Fatalf("s3compat config: %+v", c)
	}
	bad := r2Env()
	bad["MEDIA_R2_ACCOUNT_ID"] = "x.r2.cloudflarestorage.com"
	if _, err := envFrom(bad); err == nil {
		t.Error("account id with domain accepted")
	}
	bad = s3Env()
	bad["MEDIA_S3_ENDPOINT"] = "localhost:9000"
	if _, err := envFrom(bad); err == nil {
		t.Error("endpoint without scheme accepted")
	}
}

func TestNewRejectsIncompleteConfig(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Error("zero config accepted")
	}
	c, _ := envFrom(s3Env())
	c.Bucket = ""
	if _, err := New(c); err == nil {
		t.Error("empty bucket accepted")
	}
}

func presigned(t *testing.T, env map[string]string) (*url.URL, PresignedPut) {
	t.Helper()
	c, err := envFrom(env)
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.PresignPut(context.Background(), PresignPutInput{
		Key: NewKey(PrefixTmp), ContentLength: 2000, ContentType: "image/png",
		ContentMD5: "1B2M2Y8AsgTpgAmY7PhCfg==", Expires: 10 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(p.URL)
	if err != nil {
		t.Fatal(err)
	}
	return u, p
}

func TestPresignedPutSignsExactlyTheBoundHeaders(t *testing.T) {
	for _, env := range []map[string]string{r2Env(), s3Env()} {
		u, p := presigned(t, env)
		q := u.Query()
		if got, want := q.Get("X-Amz-SignedHeaders"), "content-length;content-md5;content-type;host;if-none-match"; got != want {
			t.Errorf("%s SignedHeaders = %q, want %q", env["MEDIA_STORAGE"], got, want)
		}
		if q.Get("X-Amz-Signature") == "" || q.Get("X-Amz-Expires") != "600" {
			t.Errorf("query: %v", q)
		}
		if p.Headers["If-None-Match"] != "*" || p.Headers["Content-Length"] != "" ||
			p.Headers["Content-Type"] != "image/png" || p.Headers["Content-MD5"] == "" {
			t.Errorf("headers: %v", p.Headers)
		}
	}
}

func TestPresignedURLAddressing(t *testing.T) {
	u, _ := presigned(t, s3Env())
	if u.Host != "localhost:9000" || !strings.HasPrefix(u.Path, "/bkt/tmp/") {
		t.Errorf("s3compat url: host %q path %q", u.Host, u.Path)
	}
	u, _ = presigned(t, r2Env())
	if u.Host != "bkt.acct123.r2.cloudflarestorage.com" || !strings.HasPrefix(u.Path, "/tmp/") {
		t.Errorf("r2 url: host %q path %q", u.Host, u.Path)
	}
}

func TestPresignRejectsIncompleteInput(t *testing.T) {
	ok := PresignPutInput{Key: "tmp/x", ContentLength: 1, ContentType: "a/b", ContentMD5: "1B2M2Y8AsgTpgAmY7PhCfg==", Expires: time.Minute}
	mutate := []func(*PresignPutInput){
		func(i *PresignPutInput) { i.Key = "" },
		func(i *PresignPutInput) { i.ContentLength = 0 },
		func(i *PresignPutInput) { i.ContentType = "" },
		func(i *PresignPutInput) { i.ContentMD5 = "" },
		func(i *PresignPutInput) { i.Expires = 0 },
		func(i *PresignPutInput) { i.Expires = MaxPresignExpiry + time.Second },
		func(i *PresignPutInput) { i.ContentMD5 = "m" },
		func(i *PresignPutInput) { i.ContentMD5 = "1B2M2Y8AsgTpgAmY7PhCfg" },
		func(i *PresignPutInput) { i.ContentMD5 = "1B2M2Y8AsgTpgAmY7PhCfg--" },
		func(i *PresignPutInput) { i.ContentMD5 = "AAAA" },
	}
	if _, err := NewFake().PresignPut(context.Background(), ok); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}
	atLimit := ok
	atLimit.Expires = MaxPresignExpiry
	if _, err := NewFake().PresignPut(context.Background(), atLimit); err != nil {
		t.Fatalf("7 day expiry rejected: %v", err)
	}
	for i, m := range mutate {
		in := ok
		m(&in)
		if _, err := NewFake().PresignPut(context.Background(), in); err == nil {
			t.Errorf("case %d accepted", i)
		}
	}
}

func TestNewKey(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range []Prefix{PrefixTmp, PrefixPublic, PrefixSecure} {
		for range 50 {
			k := NewKey(p)
			pre, id, ok := strings.Cut(k, "/")
			if !ok || pre != string(p) || !uuidV4.MatchString(id) {
				t.Fatalf("key %q", k)
			}
			if strings.Contains(k, ".attempt-") || strings.Contains(k, ".") || seen[k] {
				t.Fatalf("key %q has a dot or repeats", k)
			}
			seen[k] = true
		}
	}
}

func TestFake(t *testing.T) {
	ctx := context.Background()
	f := NewFake()
	if _, err := f.Get(ctx, "a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get missing: %v", err)
	}
	if _, err := f.Head(ctx, "a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("head missing: %v", err)
	}
	if err := f.Delete(ctx, "a"); err != nil {
		t.Fatalf("delete missing: %v", err)
	}
	if err := f.Put(ctx, PutInput{Key: "a", Body: bytes.NewReader([]byte("hello")), Size: 5, ContentType: "text/plain"}); err != nil {
		t.Fatal(err)
	}
	if err := f.Put(ctx, PutInput{Key: "b", Body: bytes.NewReader([]byte("hi")), Size: 5}); !errors.Is(err, ErrSizeMismatch) {
		t.Errorf("size mismatch: %v", err)
	}
	if err := f.Put(ctx, PutInput{Key: "c", Body: strings.NewReader("x"), Size: 1}); err != nil {
		t.Fatal(err)
	}
	if info, _ := f.Head(ctx, "c"); info.ContentType != DefaultContentType {
		t.Errorf("default content type = %q", info.ContentType)
	}
	_ = f.Delete(ctx, "c")
	info, err := f.Head(ctx, "a")
	if err != nil || info.Size != 5 || info.ContentType != "text/plain" {
		t.Fatalf("head: %+v %v", info, err)
	}
	r, err := f.Get(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(r)
	_ = r.Close()
	if string(data) != "hello" {
		t.Fatalf("get: %q", data)
	}
	if err := f.Put(ctx, PutInput{Key: "a", Body: strings.NewReader("bye"), Size: 3}); err != nil {
		t.Fatal(err)
	}
	if info, _ := f.Head(ctx, "a"); info.Size != 3 {
		t.Fatalf("overwrite: %+v", info)
	}
	if err := f.Delete(ctx, "a"); err != nil || len(f.Keys()) != 0 {
		t.Fatalf("delete: %v %v", err, f.Keys())
	}
}

func TestSecureChunkKeys(t *testing.T) {
	asset := uuid.New()
	if got, want := SecurePrefix(asset), "secure/"+asset.String()+"/"; got != want {
		t.Fatalf("SecurePrefix = %q，預期 %q", got, want)
	}
	seen := map[string]bool{}
	for index := range 3 {
		for range 20 {
			k := NewSecureChunkKey(asset, index)
			rest, ok := strings.CutPrefix(k, SecurePrefix(asset))
			idx, attempt, cut := strings.Cut(rest, "/")
			if !ok || !cut || idx != string(rune('0'+index)) || !uuidV4.MatchString(attempt) || seen[k] {
				t.Fatalf("key %q（段 %d）", k, index)
			}
			seen[k] = true
		}
	}
}

func TestFakeList(t *testing.T) {
	ctx := context.Background()
	f := NewFake()
	for _, k := range []string{"secure/a/1/x", "secure/a/0/y", "secure/b/0/z", "public/q"} {
		if err := f.Put(ctx, PutInput{Key: k, Body: strings.NewReader("x"), Size: 1}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := f.List(ctx, "secure/a/")
	if err != nil || !slices.Equal(got, []string{"secure/a/0/y", "secure/a/1/x"}) {
		t.Fatalf("List = %v, %v", got, err)
	}
	if none, err := f.List(ctx, "secure/none/"); err != nil || none == nil || len(none) != 0 {
		t.Fatalf("沒有符合的應回空 slice：%v %v", none, err)
	}
}
