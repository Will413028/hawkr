package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type fakeSecure struct {
	chunkErr   error
	chunkRead  int64
	chunkWho   media.Uploader
	chunkToken string
	chunkIndex int
	chunkLen   int64
	previewWho media.Uploader
	preview    string
	previewErr error
	// previewReader 非 nil 時取代 preview 當讀取來源。
	previewReader io.Reader
}

func (f *fakeSecure) PutSecureChunk(_ context.Context, u media.Uploader, token string, index int, body io.Reader, length int64) (media.SecureChunkResult, error) {
	f.chunkWho, f.chunkToken, f.chunkIndex, f.chunkLen = u, token, index, length
	n, err := io.Copy(io.Discard, body)
	f.chunkRead = n
	if err != nil {
		return media.SecureChunkResult{}, err
	}
	return media.SecureChunkResult{}, f.chunkErr
}

func (f *fakeSecure) ReadStagedSecure(_ context.Context, u media.Uploader, _ string) (io.ReadCloser, error) {
	f.previewWho = u
	if f.previewErr != nil {
		return nil, f.previewErr
	}
	if u.ID != 7 {
		return nil, apperr.NotFound("找不到")
	}
	if f.previewReader != nil {
		return io.NopCloser(f.previewReader), nil
	}
	return io.NopCloser(strings.NewReader(f.preview)), nil
}

func secureServer(t *testing.T, f *fakeSecure, timeout time.Duration) *httptest.Server {
	t.Helper()
	h := NewSecureUploads(f, nil)
	h.readTimeout = timeout
	mux := http.NewServeMux()
	mux.Handle(ChunkRoute, h.Chunk())
	mux.Handle(PreviewRoute, h.Preview())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("X-Test-Who") {
		case "account":
			r = r.WithContext(rpc.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalAccount, ID: 7}))
		case "other":
			r = r.WithContext(rpc.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalAccount, ID: 8}))
		case "admin":
			r = r.WithContext(rpc.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalAdmin, ID: 7}))
		}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// reply 是已讀完並關閉 body 的回應。
type reply struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

func do(t *testing.T, method, url, who string, body io.Reader, length int64) reply {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, url, body)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.ContentLength = length
	}
	if who != "" {
		req.Header.Set("X-Test-Who", who)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return reply{StatusCode: resp.StatusCode, Header: resp.Header, Body: b}
}

func TestChunkPassesRawBodyToModule(t *testing.T) {
	f := &fakeSecure{}
	srv := secureServer(t, f, time.Minute)
	resp := do(t, http.MethodPut, srv.URL+"/api/files/secure_uploads/tok/chunks/2", "account", strings.NewReader("abcde"), 5)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("狀態 %d，預期 204", resp.StatusCode)
	}
	if f.chunkToken != "tok" || f.chunkIndex != 2 || f.chunkLen != 5 || f.chunkRead != 5 || f.chunkWho.ID != 7 {
		t.Fatalf("模組收到 %+v", f)
	}
}

func TestChunkRequiresAccount(t *testing.T) {
	f := &fakeSecure{}
	srv := secureServer(t, f, time.Minute)
	for who, want := range map[string]int{"": http.StatusUnauthorized, "admin": http.StatusForbidden} {
		resp := do(t, http.MethodPut, srv.URL+"/api/files/secure_uploads/tok/chunks/0", who, strings.NewReader("a"), 1)
		if resp.StatusCode != want {
			t.Errorf("who=%q 回 %d，預期 %d", who, resp.StatusCode, want)
		}
	}
	if f.chunkToken != "" {
		t.Fatal("未授權不該到模組")
	}
}

// 宣告的長度在上限內、實際 body 超過：由 MaxBytesReader 擋下（直接呼叫 handler，真的 client 不會多送）。
func TestChunkOverSegmentSizeIsRejected(t *testing.T) {
	f := &fakeSecure{}
	h := NewSecureUploads(f, nil).Chunk()
	size := int64(media.SecureChunkSize + 1)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/files/secure_uploads/tok/chunks/0", io.LimitReader(zeroReader{}, size))
	req.ContentLength = 1
	req.SetPathValue("token", "tok")
	req.SetPathValue("index", "0")
	req = req.WithContext(rpc.WithPrincipal(req.Context(), identity.Principal{Kind: identity.PrincipalAccount, ID: 7}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("狀態 %d，預期 413", rec.Code)
	}
	if f.chunkRead > media.SecureChunkSize {
		t.Fatalf("模組讀到 %d 位元組，超過段大小", f.chunkRead)
	}
}

// 宣告的 Content-Length 超過段大小：在進模組前就回 413。
func TestChunkDeclaredLengthOverSegmentSizeIs413(t *testing.T) {
	f := &fakeSecure{}
	srv := secureServer(t, f, time.Minute)
	size := int64(media.SecureChunkSize + 1)
	resp := do(t, http.MethodPut, srv.URL+"/api/files/secure_uploads/tok/chunks/0", "account", io.LimitReader(zeroReader{}, size), size)
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("狀態 %d，預期 413", resp.StatusCode)
	}
	if f.chunkToken != "" {
		t.Fatal("超過段大小的宣告不該到模組")
	}
}

func TestChunkNonCanonicalIndexIs404(t *testing.T) {
	f := &fakeSecure{}
	srv := secureServer(t, f, time.Minute)
	for _, idx := range []string{"+1", "01", "-1", "1.0", "x"} {
		resp := do(t, http.MethodPut, srv.URL+"/api/files/secure_uploads/tok/chunks/"+idx, "account", strings.NewReader("a"), 1)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("index %q 回 %d，預期 404", idx, resp.StatusCode)
		}
	}
	if f.chunkToken != "" {
		t.Fatal("不合規的 index 不該到模組")
	}
}

type logSink struct {
	mu sync.Mutex
	n  int
}

func (l *logSink) Enabled(context.Context, slog.Level) bool { return true }
func (l *logSink) Handle(_ context.Context, r slog.Record) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if r.Level >= slog.LevelError {
		l.n++
	}
	return nil
}
func (l *logSink) WithAttrs([]slog.Attr) slog.Handler { return l }
func (l *logSink) WithGroup(string) slog.Handler      { return l }

func chunkRequest(body io.Reader) *http.Request {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/x", body)
	req.ContentLength = 4
	req.SetPathValue("token", "tok")
	req.SetPathValue("index", "0")
	return req.WithContext(rpc.WithPrincipal(req.Context(), identity.Principal{Kind: identity.PrincipalAccount, ID: 7}))
}

// 錯誤出自讀 body（client 斷線）：400，不記 error。
func TestChunkClientAbortIs400WithoutErrorLog(t *testing.T) {
	for _, err := range []error{
		context.Canceled,
		io.ErrUnexpectedEOF,
		syscall.ECONNRESET,
		syscall.EPIPE,
		net.ErrClosed,
	} {
		sink := &logSink{}
		h := NewSecureUploads(&fakeSecure{}, slog.New(sink)).Chunk()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, chunkRequest(io.MultiReader(strings.NewReader("ab"), errReader{err: err})))
		if rec.Code != http.StatusBadRequest || sink.n != 0 {
			t.Errorf("%v：狀態 %d、error log %d 筆，預期 400 與 0 筆", err, rec.Code, sink.n)
		}
	}
}

// 儲存寫入的 EPIPE（不是讀 body）是伺服器的錯：500 並記 error。
func TestChunkStorageWriteErrorIs500WithErrorLog(t *testing.T) {
	for _, err := range []error{syscall.EPIPE, syscall.ECONNRESET, io.ErrUnexpectedEOF, net.ErrClosed} {
		sink := &logSink{}
		h := NewSecureUploads(&fakeSecure{chunkErr: fmt.Errorf("寫暫存：%w", err)}, slog.New(sink)).Chunk()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, chunkRequest(strings.NewReader("abcd")))
		if rec.Code != http.StatusInternalServerError || sink.n != 1 {
			t.Errorf("%v：狀態 %d、error log %d 筆，預期 500 與 1 筆", err, rec.Code, sink.n)
		}
	}
}

// 預覽沒有 body：儲存的 ECONNRESET 是伺服器的錯，500 並記 error。
func TestPreviewStorageResetIs500WithErrorLog(t *testing.T) {
	sink := &logSink{}
	h := NewSecureUploads(&fakeSecure{previewErr: fmt.Errorf("讀暫存：%w", syscall.ECONNRESET)}, slog.New(sink)).Preview()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", nil)
	req.SetPathValue("token", "tok")
	req = req.WithContext(rpc.WithPrincipal(req.Context(), identity.Principal{Kind: identity.PrincipalAccount, ID: 7}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError || sink.n != 1 {
		t.Errorf("狀態 %d、error log %d 筆，預期 500 與 1 筆", rec.Code, sink.n)
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

func TestChunkSlowBodyTimesOut(t *testing.T) {
	f := &fakeSecure{}
	srv := secureServer(t, f, 150*time.Millisecond)
	pr, pw := io.Pipe()
	t.Cleanup(func() { _ = pw.Close() })
	go func() { _, _ = pw.Write([]byte("ab")) }()
	done := make(chan reply, 1)
	go func() {
		done <- do(t, http.MethodPut, srv.URL+"/api/files/secure_uploads/tok/chunks/0", "account", pr, 10)
	}()
	select {
	case resp := <-done:
		if resp.StatusCode != http.StatusRequestTimeout {
			t.Fatalf("狀態 %d，預期 408", resp.StatusCode)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("慢速 body 沒有在時限內被切斷")
	}
}

func TestChunkIndexMismatchIs409WithNextIndex(t *testing.T) {
	f := &fakeSecure{chunkErr: apperr.FailedPrecondition("下一段是 3").WithMetadata(metaNextIndex, "3")}
	srv := secureServer(t, f, time.Minute)
	resp := do(t, http.MethodPut, srv.URL+"/api/files/secure_uploads/tok/chunks/1", "account", strings.NewReader("a"), 1)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("狀態 %d，預期 409", resp.StatusCode)
	}
	if got := resp.Header.Get("Hawkr-Next-Index"); got != "3" {
		t.Fatalf("Hawkr-Next-Index = %q，預期 3", got)
	}
	if strings.Contains(string(resp.Body), "3") {
		t.Fatalf("body 不該帶細節：%q", resp.Body)
	}
}

func TestChunkOtherFailuresAreStatusOnly(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{apperr.FailedPrecondition("過期"), http.StatusPreconditionFailed},
		{apperr.NotFound("沒有"), http.StatusNotFound},
		{apperr.InvalidInput("長度"), http.StatusBadRequest},
		{apperr.ResourceExhausted(5*time.Second, "忙"), http.StatusServiceUnavailable},
		{media.ErrNoKeyring, http.StatusServiceUnavailable},
	}
	for _, c := range cases {
		f := &fakeSecure{chunkErr: c.err}
		srv := secureServer(t, f, time.Minute)
		resp := do(t, http.MethodPut, srv.URL+"/api/files/secure_uploads/tok/chunks/0", "account", strings.NewReader("a"), 1)
		if resp.StatusCode != c.want {
			t.Errorf("%v 回 %d，預期 %d", c.err, resp.StatusCode, c.want)
		}
		if resp.Header.Get("Hawkr-Next-Index") != "" {
			t.Errorf("%v 不該帶 Hawkr-Next-Index", c.err)
		}
	}
}

func TestPreviewOnlyForTheUploader(t *testing.T) {
	f := &fakeSecure{preview: "%PDF-1.4 hello"}
	srv := secureServer(t, f, time.Minute)
	own := do(t, http.MethodGet, srv.URL+"/api/files/secure_uploads/tok", "account", nil, 0)
	if own.StatusCode != http.StatusOK || string(own.Body) != "%PDF-1.4 hello" {
		t.Fatalf("本人：%d %q", own.StatusCode, own.Body)
	}
	if ct := own.Header.Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("Content-Type = %q", ct)
	}
	if own.Header.Get("Cache-Control") != "private, no-store" || own.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("機敏 header 缺：%v", own.Header)
	}
	if f.previewWho.ID != 7 {
		t.Fatalf("模組收到的上傳者 %v，預期 7（來自 principal，不是請求）", f.previewWho)
	}
	for who, want := range map[string]int{"other": http.StatusNotFound, "": http.StatusUnauthorized, "admin": http.StatusForbidden} {
		resp := do(t, http.MethodGet, srv.URL+"/api/files/secure_uploads/tok", who, nil, 0)
		if resp.StatusCode != want {
			t.Errorf("who=%q 回 %d，預期 %d", who, resp.StatusCode, want)
		}
	}
}

func TestPreviewUnknownTypeIsOpaque(t *testing.T) {
	f := &fakeSecure{preview: "<html><script>alert(1)</script></html>"}
	srv := secureServer(t, f, time.Minute)
	resp := do(t, http.MethodGet, srv.URL+"/api/files/secure_uploads/tok", "account", nil, 0)
	if ct := resp.Header.Get("Content-Type"); ct != "application/octet-stream" {
		t.Fatalf("Content-Type = %q，預期 application/octet-stream", ct)
	}
}

// 預覽讀不到第一段：在送 header 前回錯誤狀態，不是帶著壞內容的 200。
func TestPreviewFirstSegmentFailureIsNot200(t *testing.T) {
	f := &fakeSecure{previewReader: errReader{err: errors.New("kek 解不開")}}
	srv := secureServer(t, f, time.Minute)
	resp := do(t, http.MethodGet, srv.URL+"/api/files/secure_uploads/tok", "account", nil, 0)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("狀態 %d，預期 500", resp.StatusCode)
	}
}

// 預覽送出中途失敗：連線被中止，client 讀不到完整回應。
func TestPreviewMidStreamFailureAbortsResponse(t *testing.T) {
	head := strings.Repeat("a", 2<<20)
	f := &fakeSecure{previewReader: io.MultiReader(strings.NewReader(head), errReader{err: errors.New("第二段壞了")})}
	srv := secureServer(t, f, time.Minute)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/files/secure_uploads/tok", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Test-Who", "account")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do 應成功（header 已送出）：%v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("狀態 %d，預期 200", resp.StatusCode)
	}
	if _, err := io.ReadAll(resp.Body); err == nil {
		t.Fatal("中途失敗的預期連線被中止，卻讀到完整回應")
	}
}

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }
