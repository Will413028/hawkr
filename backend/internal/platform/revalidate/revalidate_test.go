package revalidate

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

func job(tags ...string) *river.Job[Args] {
	return &river.Job[Args]{JobRow: &rivertype.JobRow{Attempt: 1}, Args: Args{Tags: tags}}
}

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func isCancel(err error) bool {
	var c *river.JobCancelError
	return errors.As(err, &c)
}

func TestWorkPostsTagsWithTheSecret(t *testing.T) {
	var got struct {
		path, secret, contentType string
		body                      Args
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.path, got.secret, got.contentType = r.URL.Path, r.Header.Get("x-revalidate-secret"), r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&got.body); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()

	w := NewWorker(Config{BaseURL: srv.URL, Secret: "s3cret"}, quietLogger())
	postID := uuid.MustParse("01998a3c-7d2e-7b10-8c4f-3a5e6d7f8091")
	if err := w.Work(context.Background(), job(PostTag(postID))); err != nil {
		t.Fatal(err)
	}
	if got.path != "/api/revalidate" || got.secret != "s3cret" || got.contentType != "application/json" {
		t.Errorf("請求 = %+v", got)
	}
	if !slices.Equal(got.body.Tags, []string{"post:01998a3c-7d2e-7b10-8c4f-3a5e6d7f8091"}) {
		t.Errorf("tags = %v", got.body.Tags)
	}
}

// 5xx 與 429 是暫時的，交給 River 退避重試；其他非 2xx 是設定錯誤，重試也不會好。
func TestWorkRetriesOnlyTransientStatuses(t *testing.T) {
	for _, c := range []struct {
		status int
		retry  bool
	}{
		{http.StatusInternalServerError, true},
		{http.StatusBadGateway, true},
		{http.StatusServiceUnavailable, true},
		{http.StatusTooManyRequests, true},
		{http.StatusBadRequest, false},
		{http.StatusUnauthorized, false}, // secret 不符
		{http.StatusNotFound, false},     // 網址設錯
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "x", c.status)
		}))
		err := NewWorker(Config{BaseURL: srv.URL, Secret: "x"}, quietLogger()).Work(context.Background(), job("post:1"))
		srv.Close()
		if err == nil {
			t.Fatalf("status %d：預期失敗", c.status)
		}
		if isCancel(err) == c.retry {
			t.Errorf("status %d：取消 = %v，預期重試 = %v（err = %v）", c.status, isCancel(err), c.retry, err)
		}
	}
}

func TestWorkRetriesConnectionErrors(t *testing.T) {
	ln, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close() // 沒有人在聽：連線被拒

	err = NewWorker(Config{BaseURL: "http://" + addr, Secret: "x"}, quietLogger()).Work(context.Background(), job("post:1"))
	if err == nil || isCancel(err) {
		t.Fatalf("連線層錯誤應回傳可重試的 error；err = %v", err)
	}
}

func TestWorkRetriesTimeouts(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		<-release // 一直不回應，直到測試結束
	}))
	defer srv.Close()
	defer close(release) // 先於 srv.Close 執行：Close 會等所有 handler 返回

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := NewWorker(Config{BaseURL: srv.URL, Secret: "x"}, quietLogger()).Work(ctx, job("post:1"))
	if err == nil || isCancel(err) {
		t.Fatalf("timeout 應回傳可重試的 error；err = %v", err)
	}
}

func TestWorkIsANoOpWhenDisabled(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer srv.Close()

	if err := NewWorker(Config{BaseURL: srv.URL}, quietLogger()).Work(context.Background(), job("post:1")); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("沒有 secret 時不應送出")
	}
}

// 重試用 River 預設的退避（NextRetry 回零值），次數讓最後一次落在貼文相關快取最長的 TTL 之內。
func TestRetriesBackOffWithinTheCacheTTL(t *testing.T) {
	if got := (Args{}).InsertOpts().MaxAttempts; got != 5 {
		t.Fatalf("MaxAttempts = %d，預期 5", got)
	}
	if next := NewWorker(Config{}, quietLogger()).NextRetry(job("post:1")); !next.IsZero() {
		t.Fatalf("NextRetry = %v，應交給 River 預設的退避", next)
	}
}

func TestPostTagIsAnEntityTagWebsiteAccepts(t *testing.T) {
	id := uuid.NewV7()
	tag := PostTag(id)
	if tag != "post:"+id.String() {
		t.Fatalf("PostTag = %q", tag)
	}
	if !regexp.MustCompile(`^(?:product|seller|post|category|hashtag):[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`).MatchString(tag) {
		t.Errorf("%q 不符合 website 的 ENTITY_TAG（revalidate-request.ts）", tag)
	}
}
