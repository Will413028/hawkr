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

	httpdelegation "github.com/Will413028/hawkr/backend/internal/httpapi/delegation"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	moddelegation "github.com/Will413028/hawkr/backend/internal/modules/delegation"
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
	if who == "delegated" {
		req.Header.Set(httpdelegation.OperationSessionHeader, "cred")
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

func delegatedSecureServer(t *testing.T, f *fakeSecure, timeout time.Duration) *httptest.Server {
	t.Helper()
	return delegatedSecureServerWith(t, f, timeout, delegatedSessions())
}

// delegatedSessions 是代行路由預設的代行 session：憑證 "cred" 是管理員 7 在店家 5 上的 ACTIVE session，帶 media.upload。
func delegatedSessions() *fakeSessions {
	return &fakeSessions{sessions: map[string]moddelegation.DelegationSession{
		"cred": {ID: 9, Status: moddelegation.DelegationActive, Capabilities: []moddelegation.Capability{moddelegation.CapabilityMediaUpload},
			Actor: moddelegation.DelegationActor{AdminID: 7}, Target: moddelegation.DelegationTarget{StoreID: 5}},
	}}
}

func delegatedSecureServerWith(t *testing.T, f *fakeSecure, timeout time.Duration, sessions DelegationAccess) *httptest.Server {
	t.Helper()
	h := NewSecureUploads(f, nil)
	if sessions != nil {
		h.WithDelegation(sessions)
	}
	h.readTimeout = timeout
	mux := http.NewServeMux()
	mux.Handle(DelegatedChunkRoute, h.DelegatedChunk())
	mux.Handle(DelegatedPreviewRoute, h.DelegatedPreview())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("X-Test-Who") {
		case "account":
			r = r.WithContext(rpc.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalAccount, ID: 7}))
		case "admin":
			r = r.WithContext(rpc.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalAdmin, ID: 7}))
		}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// 代行的段上傳：上傳者來自 session（{ID: session 的管理員, Admin: true, Store: session 的店家}），原始位元組原樣交給模組；
// 只帶顧客帳號或管理員身分、沒有代行憑證的請求被拒絕，模組沒有被呼叫；成功的請求順延 session 的閒置逾時。
func TestDelegatedChunkUploaderComesFromTheSession(t *testing.T) {
	f := &fakeSecure{}
	sessions := delegatedSessions()
	srv := delegatedSecureServerWith(t, f, time.Minute, sessions)
	resp := do(t, http.MethodPut, srv.URL+"/api/files/delegated/secure_uploads/tok/chunks/2", "delegated", strings.NewReader("abcde"), 5)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("狀態 %d，預期 204", resp.StatusCode)
	}
	if f.chunkWho != (media.Uploader{ID: 7, Admin: true, Store: 5}) || f.chunkToken != "tok" || f.chunkIndex != 2 || f.chunkLen != 5 || f.chunkRead != 5 {
		t.Fatalf("模組收到 %+v", f)
	}
	if h := resp.Header.Get("Cache-Control"); h != "private, no-store" {
		t.Fatalf("Cache-Control = %q", h)
	}
	if len(sessions.touched) != 1 || sessions.touched[0] != 9 {
		t.Fatalf("順延閒置逾時 %v，預期 [9]", sessions.touched)
	}
	f = &fakeSecure{}
	sessions = delegatedSessions()
	srv = delegatedSecureServerWith(t, f, time.Minute, sessions)
	for _, who := range []string{"", "account", "admin"} {
		resp := do(t, http.MethodPut, srv.URL+"/api/files/delegated/secure_uploads/tok/chunks/0", who, strings.NewReader("a"), 1)
		if resp.StatusCode != http.StatusUnauthorized || f.chunkToken != "" || len(sessions.touched) != 0 {
			t.Errorf("who=%q 回 %d（模組被呼叫：%v），預期 401 且不呼叫", who, resp.StatusCode, f.chunkToken != "")
		}
	}
}

// 代行路由沿用顧客段路由的語意：超過段大小 413、index 不符 409 加 Hawkr-Next-Index、慢速 body 408。
func TestDelegatedChunkSharesTheChunkSemantics(t *testing.T) {
	url := "/api/files/delegated/secure_uploads/tok/chunks/0"
	big := strings.NewReader(strings.Repeat("a", media.SecureChunkSize+1))
	srv := delegatedSecureServer(t, &fakeSecure{}, time.Minute)
	if resp := do(t, http.MethodPut, srv.URL+url, "delegated", big, media.SecureChunkSize+1); resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("超量回 %d，預期 413", resp.StatusCode)
	}
	mismatch := apperr.FailedPrecondition("不符").WithMetadata(metaNextIndex, "3")
	sessions := delegatedSessions()
	srv = delegatedSecureServerWith(t, &fakeSecure{chunkErr: mismatch}, time.Minute, sessions)
	resp := do(t, http.MethodPut, srv.URL+url, "delegated", strings.NewReader("a"), 1)
	if resp.StatusCode != http.StatusConflict || resp.Header.Get(NextIndexHeader) != "3" {
		t.Errorf("index 不符回 %d、%s=%q，預期 409 與 3", resp.StatusCode, NextIndexHeader, resp.Header.Get(NextIndexHeader))
	}
	if len(sessions.touched) != 0 {
		t.Errorf("失敗的請求不算活動，卻順延了 %v", sessions.touched)
	}
	srv = delegatedSecureServer(t, &fakeSecure{}, 100*time.Millisecond)
	pr, pw := io.Pipe()
	t.Cleanup(func() { _ = pw.Close() })
	go func() { _, _ = pw.Write([]byte("ab")) }()
	if resp := do(t, http.MethodPut, srv.URL+url, "delegated", pr, 10); resp.StatusCode != http.StatusRequestTimeout {
		t.Errorf("慢速 body 回 %d，預期 408", resp.StatusCode)
	}
}

func delegatedPut(t *testing.T, srv *httptest.Server, cred string) reply {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, srv.URL+"/api/files/delegated/secure_uploads/tok/chunks/0", strings.NewReader("a"))
	if err != nil {
		t.Fatal(err)
	}
	if cred != "" {
		req.Header.Set(httpdelegation.OperationSessionHeader, cred)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	return reply{StatusCode: resp.StatusCode, Header: resp.Header}
}

// 代行段上傳要有效的代行 session：缺憑證與無效憑證 401；已撤銷、已逾時、被移除 media.upload 的 session 403；模組都沒被呼叫，被拒絕的 session 記下稽核宣告。
func TestDelegatedChunkRequiresALiveDelegationSession(t *testing.T) {
	mutate := func(f func(s *moddelegation.DelegationSession)) *fakeSessions {
		fs := delegatedSessions()
		s := fs.sessions["cred"]
		f(&s)
		fs.sessions["cred"] = s
		return fs
	}
	for name, tc := range map[string]struct {
		sessions *fakeSessions
		cred     string
		want     int
		denied   int
	}{
		"缺憑證":  {delegatedSessions(), "", http.StatusUnauthorized, 0},
		"無效憑證": {delegatedSessions(), "bad", http.StatusUnauthorized, 0},
		"已撤銷":  {mutate(func(s *moddelegation.DelegationSession) { s.Status = moddelegation.DelegationRevoked }), "cred", http.StatusForbidden, 1},
		"已逾時":  {mutate(func(s *moddelegation.DelegationSession) { s.Status = moddelegation.DelegationExpired }), "cred", http.StatusForbidden, 1},
		"移除了 media.upload": {mutate(func(s *moddelegation.DelegationSession) {
			s.Capabilities = []moddelegation.Capability{moddelegation.CapabilityStorefrontWrite}
		}), "cred", http.StatusForbidden, 1},
		"有效": {delegatedSessions(), "cred", http.StatusNoContent, 0},
	} {
		f := &fakeSecure{}
		resp := delegatedPut(t, delegatedSecureServerWith(t, f, time.Minute, tc.sessions), tc.cred)
		called := f.chunkToken != ""
		if resp.StatusCode != tc.want || called != (tc.want == http.StatusNoContent) || len(tc.sessions.denied) != tc.denied {
			t.Errorf("%s：狀態 %d、模組被呼叫 %v、稽核宣告 %d，預期 %d、%v、%d", name, resp.StatusCode, called, len(tc.sessions.denied), tc.want, tc.want == http.StatusNoContent, tc.denied)
		}
		if tc.denied == 1 && len(tc.sessions.denied) == 1 && tc.sessions.denied[0].Action != moddelegation.AuditMediaUpload {
			t.Errorf("%s：稽核動作 %q", name, tc.sessions.denied[0].Action)
		}
	}
	if resp := delegatedPut(t, delegatedSecureServerWith(t, &fakeSecure{}, time.Minute, nil), "cred"); resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("沒有接上 delegation：%d，預期 503", resp.StatusCode)
	}
}

// 代行預覽：上傳者同為 session 的管理員加店家；沒有有效 session 一律被擋；只帶顧客帳號沒有憑證 401；成功的預覽順延閒置逾時。
func TestDelegatedPreviewUsesTheSessionUploader(t *testing.T) {
	f := &fakeSecure{preview: "%PDF-1.4 hi"}
	sessions := delegatedSessions()
	srv := delegatedSecureServerWith(t, f, time.Minute, sessions)
	resp := do(t, http.MethodGet, srv.URL+"/api/files/delegated/secure_uploads/tok", "delegated", nil, 0)
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "application/pdf" || resp.Header.Get("Cache-Control") != "private, no-store" {
		t.Fatalf("預覽 %d %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	if f.previewWho != (media.Uploader{ID: 7, Admin: true, Store: 5}) {
		t.Fatalf("模組收到 %+v", f.previewWho)
	}
	if len(sessions.touched) != 1 || sessions.touched[0] != 9 {
		t.Fatalf("順延閒置逾時 %v，預期 [9]", sessions.touched)
	}
	f = &fakeSecure{preview: "x"}
	srv = delegatedSecureServerWith(t, f, time.Minute, func() *fakeSessions {
		fs := delegatedSessions()
		s := fs.sessions["cred"]
		s.Status = moddelegation.DelegationRevoked
		fs.sessions["cred"] = s
		return fs
	}())
	if resp := do(t, http.MethodGet, srv.URL+"/api/files/delegated/secure_uploads/tok", "delegated", nil, 0); resp.StatusCode != http.StatusForbidden || f.previewWho != (media.Uploader{}) {
		t.Errorf("撤銷後預覽 %d、模組被呼叫 %+v", resp.StatusCode, f.previewWho)
	}
	srv = delegatedSecureServer(t, &fakeSecure{}, time.Minute)
	if resp := do(t, http.MethodGet, srv.URL+"/api/files/delegated/secure_uploads/tok", "account", nil, 0); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("顧客帳號預覽 %d，預期 401", resp.StatusCode)
	}
}

// 店家此刻不可代行（關店、停權、撤回代行授權；模組回 FailedPrecondition）：續傳與預覽都 403、模組沒被呼叫、恰好一列 FAILED 稽核、不順延閒置逾時。
func TestDelegatedRoutesRejectAStoreThatIsNoLongerDelegable(t *testing.T) {
	sessions := delegatedSessions()
	sessions.storeErr = apperr.FailedPrecondition("此店家目前不可代行")
	f := &fakeSecure{}
	srv := delegatedSecureServerWith(t, f, time.Minute, sessions)
	if resp := delegatedPut(t, srv, "cred"); resp.StatusCode != http.StatusForbidden {
		t.Errorf("續傳 %d，預期 403", resp.StatusCode)
	}
	if len(sessions.denied) != 1 || sessions.denied[0].Action != moddelegation.AuditMediaUpload {
		t.Errorf("續傳的 FAILED 稽核 %v，預期恰好一列 MEDIA_UPLOAD", sessions.denied)
	}
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/files/delegated/secure_uploads/tok", nil)
	req.Header.Set(httpdelegation.OperationSessionHeader, "cred")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("預覽 %d，預期 403", resp.StatusCode)
	}
	if len(sessions.denied) != 2 || f.chunkToken != "" || len(sessions.touched) != 0 {
		t.Errorf("稽核 %d 列、模組被呼叫 %q、順延 %v，預期 2 列、未呼叫、未順延", len(sessions.denied), f.chunkToken, sessions.touched)
	}
}
