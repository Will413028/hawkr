package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

type fakeS3 struct {
	srv      *httptest.Server
	requests atomic.Int32
}

func newFakeS3(t *testing.T, handler http.HandlerFunc) (*S3, *fakeS3) {
	t.Helper()
	f := &fakeS3{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests.Add(1)
		handler(w, r)
	}))
	t.Cleanup(f.srv.Close)
	env := s3Env()
	env["MEDIA_S3_ENDPOINT"] = f.srv.URL
	c, err := envFrom(env)
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	return s, f
}

func s3Error(code string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>`+code+`</Code><Message>m</Message></Error>`)
	}
}

func TestS3GetMapsOnlyMissingKeyToNotFound(t *testing.T) {
	s, _ := newFakeS3(t, s3Error("NoSuchKey"))
	if _, err := s.Get(context.Background(), "k"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("NoSuchKey: %v", err)
	}
	s, _ = newFakeS3(t, s3Error("NoSuchBucket"))
	if _, err := s.Get(context.Background(), "k"); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("NoSuchBucket must be a plain error: %v", err)
	}
}

func TestS3HeadMapsOnlyMissingKeyToNotFound(t *testing.T) {
	s, _ := newFakeS3(t, s3Error("ignored"))
	if _, err := s.Head(context.Background(), "k"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("HEAD 404 without body: %v", err)
	}
}

func TestS3GetAndHeadReadObject(t *testing.T) {
	s, _ := newFakeS3(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", "5")
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, "hello")
		}
	})
	info, err := s.Head(context.Background(), "k")
	if err != nil || info.Size != 5 || info.ContentType != "image/png" {
		t.Fatalf("head: %+v %v", info, err)
	}
	r, err := s.Get(context.Background(), "k")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(r)
	_ = r.Close()
	if string(data) != "hello" {
		t.Fatalf("get: %q", data)
	}
}

func TestS3DeleteIsIdempotentButSurfacesNoSuchBucket(t *testing.T) {
	s, _ := newFakeS3(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	if err := s.Delete(context.Background(), "k"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	s, _ = newFakeS3(t, s3Error("NoSuchBucket"))
	if err := s.Delete(context.Background(), "k"); err == nil {
		t.Fatal("NoSuchBucket swallowed")
	}
}

func TestS3PutSendsBodyAndType(t *testing.T) {
	var gotBody, gotType string
	s, _ := newFakeS3(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody, gotType = string(b), r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	})
	for _, body := range []io.Reader{bytes.NewReader([]byte("hello")), io.NopCloser(strings.NewReader("hello"))} {
		gotBody, gotType = "", ""
		if err := s.Put(context.Background(), PutInput{Key: "k", Body: body, Size: 5, ContentType: "text/plain"}); err != nil {
			t.Fatal(err)
		}
		if gotBody != "hello" || gotType != "text/plain" {
			t.Fatalf("server saw %q %q", gotBody, gotType)
		}
	}
}

func TestS3PutRejectsLengthMismatchBeforeSending(t *testing.T) {
	s, f := newFakeS3(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	seekable := bytes.NewReader([]byte("hello"))
	_, _ = seekable.Seek(2, io.SeekStart)
	cases := map[string]struct {
		body io.Reader
		size int64
	}{
		"seekable short":       {bytes.NewReader([]byte("hi")), 5},
		"seekable long":        {bytes.NewReader([]byte("hello world")), 5},
		"seekable from offset": {seekable, 5},
		"stream short":         {io.NopCloser(strings.NewReader("hi")), 5},
		"stream long":          {io.NopCloser(strings.NewReader("hello world")), 5},
	}
	for name, c := range cases {
		err := s.Put(context.Background(), PutInput{Key: "k", Body: c.body, Size: c.size})
		if !errors.Is(err, ErrSizeMismatch) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if n := f.requests.Load(); n != 0 {
		t.Fatalf("server received %d requests", n)
	}
	if pos, _ := seekable.Seek(0, io.SeekCurrent); pos != 2 {
		t.Fatalf("seekable body position moved to %d", pos)
	}
}
