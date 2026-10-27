package usecase

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/config"
	"github.com/Will413028/hawkr/ai-service/internal/domain"
	"github.com/stretchr/testify/require"
)

func newTestTransformUseCase(t *testing.T, handler http.HandlerFunc) *TransformUseCase {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &TransformUseCase{apiKey: "test-key", apiURL: server.URL}
}

// jsonResponder 回固定 JSON，並回報被打了幾次（重試行為的斷言依據）。
func jsonResponder(bodies ...string) (http.HandlerFunc, *atomic.Int32) {
	var calls atomic.Int32
	return func(w http.ResponseWriter, r *http.Request) {
		n := int(calls.Add(1))
		body := bodies[len(bodies)-1]
		if n <= len(bodies) {
			body = bodies[n-1]
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}, &calls
}

const (
	respNoImage      = `{"candidates":[{"finishReason":"NO_IMAGE","content":{"parts":[]}}]}`
	respImageSafety  = `{"candidates":[{"finishReason":"IMAGE_SAFETY","content":{"parts":[]}}]}`
	respRecitation   = `{"candidates":[{"finishReason":"IMAGE_RECITATION","content":{"parts":[]}}]}`
	respPromptBlock  = `{"promptFeedback":{"blockReason":"SAFETY"},"candidates":[]}`
	respEmptyUnknown = `{"candidates":[]}`
	respSuccess      = `{"candidates":[{"finishReason":"STOP","content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"YWJj"}}]}}]}`
)

func mustRefusal(t *testing.T, err error) *domain.ProviderRefusalError {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var refusal *domain.ProviderRefusalError
	if !errors.As(err, &refusal) {
		t.Fatalf("expected *domain.ProviderRefusalError, got %T: %v", err, err)
	}
	return refusal
}

func TestCallGemini_PromptFeedbackBlockedIsNotRetryable(t *testing.T) {
	handler, _ := jsonResponder(respPromptBlock)
	uc := newTestTransformUseCase(t, handler)

	_, _, err := uc.callGemini(context.Background(), []byte("fake"), "image/png", "test prompt")
	refusal := mustRefusal(t, err)
	if refusal.BlockReason != "SAFETY" {
		t.Errorf("BlockReason = %q, want SAFETY", refusal.BlockReason)
	}
	if refusal.Retryable() {
		t.Error("prompt-side block must not be retryable")
	}
}

func TestCallGemini_ImageSafetyIsNotRetryable(t *testing.T) {
	handler, _ := jsonResponder(respImageSafety)
	uc := newTestTransformUseCase(t, handler)

	_, _, err := uc.callGemini(context.Background(), []byte("fake"), "image/png", "test prompt")
	refusal := mustRefusal(t, err)
	if refusal.FinishReason != domain.FinishReasonImageSafety {
		t.Errorf("FinishReason = %q, want %q", refusal.FinishReason, domain.FinishReasonImageSafety)
	}
	if refusal.Retryable() {
		t.Error("IMAGE_SAFETY must not be retryable")
	}
}

// NO_IMAGE 是本次修正的核心案例：舊碼把它誤判成安全性阻擋，實際上重試就會成功。
func TestCallGemini_NoImageIsRetryable(t *testing.T) {
	handler, _ := jsonResponder(respNoImage)
	uc := newTestTransformUseCase(t, handler)

	_, _, err := uc.callGemini(context.Background(), []byte("fake"), "image/png", "test prompt")
	refusal := mustRefusal(t, err)
	if refusal.FinishReason != domain.FinishReasonNoImage {
		t.Errorf("FinishReason = %q, want %q", refusal.FinishReason, domain.FinishReasonNoImage)
	}
	if !refusal.Retryable() {
		t.Error("NO_IMAGE must be retryable")
	}
	if refusal.BlockReason != "" {
		t.Errorf("BlockReason = %q, want empty (NO_IMAGE 不是安全性阻擋)", refusal.BlockReason)
	}
}

func TestCallGemini_EmptyCandidatesFallsBackToImageOther(t *testing.T) {
	handler, _ := jsonResponder(respEmptyUnknown)
	uc := newTestTransformUseCase(t, handler)

	_, _, err := uc.callGemini(context.Background(), []byte("fake"), "image/png", "test prompt")
	refusal := mustRefusal(t, err)
	if got := refusal.Reason(); got != domain.FinishReasonImageOther {
		t.Errorf("Reason() = %q, want %q", got, domain.FinishReasonImageOther)
	}
	if !refusal.Retryable() {
		t.Error("unclassifiable empty result must be retryable")
	}
}

// 有 candidate 但沒有 inlineData：模型選擇不產圖。finishReason 是 STOP 也要正規化
// 成 NO_IMAGE，否則 Retryable() 會把它當確定性拒絕而放棄重試。
func TestCallGemini_CandidateWithoutImagePartNormalizesToNoImage(t *testing.T) {
	handler, _ := jsonResponder(`{"candidates":[{"finishReason":"STOP","content":{"parts":[{}]}}]}`)
	uc := newTestTransformUseCase(t, handler)

	_, _, err := uc.callGemini(context.Background(), []byte("fake"), "image/png", "test prompt")
	refusal := mustRefusal(t, err)
	if refusal.FinishReason != domain.FinishReasonNoImage {
		t.Errorf("FinishReason = %q, want %q", refusal.FinishReason, domain.FinishReasonNoImage)
	}
	if !refusal.Retryable() {
		t.Error("candidate without image part must be retryable")
	}
}

func TestCallGemini_ProviderErrorObjectIsUnavailable(t *testing.T) {
	handler, _ := jsonResponder(`{"error":{"message":"API key not valid"}}`)
	uc := newTestTransformUseCase(t, handler)

	_, _, err := uc.callGemini(context.Background(), []byte("fake"), "image/png", "test prompt")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var unavailable *domain.ProviderUnavailableError
	if !errors.As(err, &unavailable) {
		t.Fatalf("expected *domain.ProviderUnavailableError, got %T: %v", err, err)
	}
	var refusal *domain.ProviderRefusalError
	if errors.As(err, &refusal) {
		t.Error("provider error object must not be classified as a content refusal")
	}
}

func TestCallGemini_Success(t *testing.T) {
	handler, _ := jsonResponder(respSuccess)
	uc := newTestTransformUseCase(t, handler)

	data, mimeType, err := uc.callGemini(context.Background(), []byte("fake"), "image/png", "test prompt")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if data != "YWJj" || mimeType != "image/png" {
		t.Errorf("unexpected result: data=%q mimeType=%q", data, mimeType)
	}
}

func TestCallGeminiWithRetry_RescuesNoImage(t *testing.T) {
	handler, calls := jsonResponder(respNoImage, respSuccess)
	uc := newTestTransformUseCase(t, handler)

	data, _, err := uc.callGeminiWithRetry(context.Background(), []byte("fake"), "image/png", "test prompt")
	if err != nil {
		t.Fatalf("expected retry to succeed, got: %v", err)
	}
	if data != "YWJj" {
		t.Errorf("data = %q, want YWJj", data)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("provider calls = %d, want 2", got)
	}
}

func TestCallGeminiWithRetry_StopsAfterOneRetry(t *testing.T) {
	handler, calls := jsonResponder(respNoImage)
	uc := newTestTransformUseCase(t, handler)

	_, _, err := uc.callGeminiWithRetry(context.Background(), []byte("fake"), "image/png", "test prompt")
	mustRefusal(t, err)
	if got := calls.Load(); got != 2 {
		t.Errorf("provider calls = %d, want 2 (原呼叫 + 一次重試，不可無限重試)", got)
	}
}

func TestCallGeminiWithRetry_DoesNotRetrySafetyBlock(t *testing.T) {
	handler, calls := jsonResponder(respImageSafety)
	uc := newTestTransformUseCase(t, handler)

	_, _, err := uc.callGeminiWithRetry(context.Background(), []byte("fake"), "image/png", "test prompt")
	mustRefusal(t, err)
	if got := calls.Load(); got != 1 {
		t.Errorf("provider calls = %d, want 1 (確定性拒絕不可重試)", got)
	}
}

func TestCallGeminiWithRetry_DoesNotRetryRecitation(t *testing.T) {
	handler, calls := jsonResponder(respRecitation)
	uc := newTestTransformUseCase(t, handler)

	_, _, err := uc.callGeminiWithRetry(context.Background(), []byte("fake"), "image/png", "test prompt")
	mustRefusal(t, err)
	if got := calls.Load(); got != 1 {
		t.Errorf("provider calls = %d, want 1", got)
	}
}

func TestTransformOutcome(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"success", nil, "success"},
		{"refusal uses provider reason", &domain.ProviderRefusalError{FinishReason: domain.FinishReasonNoImage}, domain.FinishReasonNoImage},
		{"prompt block uses block reason", &domain.ProviderRefusalError{BlockReason: "SAFETY"}, "SAFETY"},
		{"unavailable", &domain.ProviderUnavailableError{Err: errors.New("boom")}, "unavailable"},
		{"unknown error", errors.New("boom"), "unavailable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := transformOutcome(tc.err); got != tc.want {
				t.Errorf("transformOutcome() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestCallGemini_TransportErrorDoesNotLeakAPIKey 涵蓋出口 A：provider 逾時／連線失敗時，
// Go 的 *url.Error 會把完整請求 URL 原樣嵌進 Error()（net/http 的 stripPassword 只遮
// userinfo，不動 query string），若 apiKey 是用 query string 拼進 URL 就會流進
// grpc/handler.go 的 zap.Error(err) log。用一個立刻關閉的 httptest server 製造確定性的
// dial 失敗（不靠真實網路逾時），斷言回傳的 error 字串不含 apiKey。
func TestCallGemini_TransportErrorDoesNotLeakAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.Close() // 立即關閉：對外送出的請求必定 dial 失敗，確定性觸發 *url.Error

	uc := &TransformUseCase{apiKey: "test-key", apiURL: srv.URL}

	_, _, err := uc.callGemini(context.Background(), []byte("x"), "image/png", "p")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if strings.Contains(err.Error(), "test-key") {
		t.Errorf("error leaks api key via URL: %v", err)
	}
}

// TestCallGemini_HeaderValidationErrorDoesNotLeakAPIKey 涵蓋一個由 apiKey 內容本身
// 觸發的 client.Do() 失敗。原本這支測試（TestCallGemini_NewRequestErrorDoesNotLeakAPIKey）
// 以為尾端帶換行的 apiKey 會讓 http.NewRequestWithContext 失敗，但那個假設只在憑證
// 還拼在 URL 裡時成立；改成 header 認證後 apiKey 不再進 URL，NewRequestWithContext
// 對 header value 完全不驗證（req.Header.Set 也不驗證），實測（go run 一個最小重現、
// 見 code-test-fix-report.md）確認 NewRequestWithContext 回傳 err == nil，真正失敗的
// 地方是稍後的 geminiHTTPClient.Do(req)：net/http/transport.go 的
// Transport.roundTrip 在送出前呼叫 validateHeaders()（同檔案，roundTrip 呼叫處
// 緊接在 nil Header 檢查之後），對每個 header value 跑 httpguts.ValidHeaderFieldValue，
// 帶控制字元（如 \n）的值會讓它回傳非空字串，roundTrip 因此直接回錯、被
// Client.do 包成 *url.Error（訊息形如 net/http: invalid header field value for
// "X-Goog-Api-Key"）。這條路徑走的其實是跟 TestCallGemini_TransportErrorDoesNotLeakAPIKey
// 相同的 callGemini 出口（client.Do() 失敗 → 包成 ProviderUnavailableError，訊息前綴
// 「呼叫 Gemini API 失敗」），差別是觸發原因直接來自 apiKey 內容本身而非任意連線失敗，
// 用來確認「apiKey 帶進 header 驗證失敗」這條路徑不會外洩憑證。net/http 的
// validateHeaders 刻意不把 value 放進錯誤訊息（transport.go 原文註解：
// 「Don't include the value in the error, because it may be sensitive.」），
// 所以這條路徑本來就不外洩；斷言錯誤字串不含 apiKey，且必須是
// *domain.ProviderUnavailableError（確認 callGemini 有把它包成已知型別，而不是
// 把可能含內部細節的 raw error 透傳出去）。
func TestCallGemini_HeaderValidationErrorDoesNotLeakAPIKey(t *testing.T) {
	uc := &TransformUseCase{apiKey: "test-key\n", apiURL: "https://example.invalid"}

	_, _, err := uc.callGemini(context.Background(), []byte("x"), "image/png", "p")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if strings.Contains(err.Error(), "test-key") {
		t.Errorf("error leaks api key via header validation failure: %v", err)
	}
	var unavailable *domain.ProviderUnavailableError
	if !errors.As(err, &unavailable) {
		t.Errorf("expected *domain.ProviderUnavailableError so handler.go's errors.As catches it, got %T: %v", err, err)
	}
}

// TestCallGemini_NewRequestFailureIsWrappedAsProviderUnavailable 直接鎖定
// http.NewRequestWithContext 失敗時的 wrap（不透過 apiKey 觸發，因為改成 header 認證後
// apiURL 不再含使用者可控內容，apiKey 內容不會再讓 NewRequest 失敗）：無論
// NewRequestWithContext 因為什麼原因失敗，callGemini 都必須把它包成
// *domain.ProviderUnavailableError，讓 grpc/handler.go 走已知的 errors.As 分支回傳固定
// 訊息，不把 raw err.Error()（可能含內部狀態）透傳給呼叫端。用一個含控制字元、
// url.Parse 必定失敗的 apiURL 直接觸發 NewRequestWithContext 錯誤。
func TestCallGemini_NewRequestFailureIsWrappedAsProviderUnavailable(t *testing.T) {
	uc := &TransformUseCase{apiKey: "test-key", apiURL: "https://example.invalid\n"}

	_, _, err := uc.callGemini(context.Background(), []byte("x"), "image/png", "p")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var unavailable *domain.ProviderUnavailableError
	if !errors.As(err, &unavailable) {
		t.Errorf("expected *domain.ProviderUnavailableError, got %T: %v", err, err)
	}
}

// TestCallGemini_SendsAPIKeyViaHeaderNotQuery 是前三個測試的正向對偶：那三個都是
// 否定式斷言（錯誤字串不含 key、錯誤型別正確），對「憑證有沒有真的送出、且送在
// x-goog-api-key」零覆蓋——把 transform_usecase.go 裡 req.Header.Set("x-goog-api-key", ...)
// 那行整行刪掉，前三個測試依然全線（沒有請求送出失敗，callGemini 走的是正常成功路徑，
// 跟憑證在哪裡完全無關）。比照 internal/procurement/usecase/deepseek_extractor_test.go
// 的既有 pattern，在 httptest handler 內把收到的值存進變數，斷言留到主 goroutine
// 執行——handler 跑在獨立的 goroutine（net/http.Server 每個連線各開一個），testify
// 的 require.* 失敗時呼叫 t.FailNow()，而 testing 套件文件明講 FailNow() 只能在
// 執行該測試的 goroutine 呼叫，在其他 goroutine 呼叫是未定義行為，所以不能直接在
// handler 內 require；client.Do() 回傳前 handler 必定已寫完 response，兩個
// goroutine 之間有 happens-before 保證，主 goroutine 讀 gotAPIKey/gotRawQuery
// 是安全的。RawQuery 斷言為空把「憑證不得回到 URL query」從比對 error 字串
// （間接、依賴訊息格式）升級成對送出的 request 的正向契約（直接、不依賴任何錯誤
// 訊息格式）。
func TestCallGemini_SendsAPIKeyViaHeaderNotQuery(t *testing.T) {
	var gotAPIKey, gotRawQuery string
	handler := func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("x-goog-api-key")
		gotRawQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(respSuccess))
	}
	uc := newTestTransformUseCase(t, handler)

	_, _, err := uc.callGemini(context.Background(), []byte("fake"), "image/png", "test prompt")
	require.NoError(t, err)
	require.Equal(t, "test-key", gotAPIKey, "api key must be sent via x-goog-api-key header")
	require.Empty(t, gotRawQuery, "api key must not be sent via URL query string")
}

// TestCallGemini_DoesNotFollowRedirect 是 geminiHTTPClient 的 CheckRedirect 回歸測試。
// 憑證改走 header 之後，net/http 的 sensitive header 白名單（$GOROOT/src/net/http/
// client.go，只列 Authorization／Www-Authenticate／Cookie／Cookie2／Proxy-*，不含
// X-Goog-Api-Key）不會在追蹤 redirect 時剝掉這個 header——預設行為會把它原樣帶到
// Location 指向的新 host。geminiHTTPClient 因此設定
// CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }，
// 讓 net/http 收到 3xx 時直接把該回應原樣交回呼叫端、不追蹤 redirect（行為細節見
// transform_usecase.go 中 geminiHTTPClient 定義處的註解）。現況：把這行從
// geminiHTTPClient 刪掉不會讓任何既有測試變紅，因為所有既有測試的 handler 都直接
// 回 200，從沒有測試模擬過 3xx 回應。這支測試起兩個 httptest server：第一個回 302
// 指向第二個，斷言第二個從未被打到——如果 CheckRedirect 被移除，client 會追蹤
// redirect 並把帶著 x-goog-api-key 的請求送到第二個 server，這支測試就會變紅。
// secondServerHit 用 atomic.Bool 而非一般 bool：handler 跑在獨立 goroutine，理由
// 同 TestCallGemini_SendsAPIKeyViaHeaderNotQuery 的說明；client.Do() 回傳前
// handler（如果真的被打到）必定已經跑完，所以主 goroutine 讀值仍是安全的，這裡用
// atomic 只是避免之後有人在這支測試上疊加平行請求時踩到同一個坑。
func TestCallGemini_DoesNotFollowRedirect(t *testing.T) {
	var secondServerHit atomic.Bool
	secondServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondServerHit.Store(true)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(respSuccess))
	}))
	t.Cleanup(secondServer.Close)

	firstServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, secondServer.URL, http.StatusFound)
	}))
	t.Cleanup(firstServer.Close)

	uc := &TransformUseCase{apiKey: "test-key", apiURL: firstServer.URL}

	// 回傳值本身不是這支測試的重點：CheckRedirect 回 http.ErrUseLastResponse 時，
	// 302 回應會被當成「最終回應」直接交回 callGemini，其 body（redirect 提示頁）
	// 不是合法 JSON，callGemini 會走 json.Unmarshal 失敗分支回錯——這是預期中的
	// 周邊效果，不用斷言，只確認 redirect 沒有被追蹤。
	_, _, _ = uc.callGemini(context.Background(), []byte("fake"), "image/png", "test prompt")

	if secondServerHit.Load() {
		t.Error("second server received a request; redirect was followed and the x-goog-api-key header may have leaked to it")
	}
}

// TestNewTransformUseCase_TrimsAPIKeyWhitespace 是建構子 strings.TrimSpace 的回歸測試。
// cfg.GeminiAPIKey 若帶尾端換行（常見於 echo/heredoc 寫入 env 檔的方式），
// NewTransformUseCase 必須先清理過再存進 uc.apiKey——理由見 NewTransformUseCase 定義
// 處的註解：帶著換行的 header value 會被 net/http 的 header 驗證擋下（見
// TestCallGemini_HeaderValidationErrorDoesNotLeakAPIKey）。TrimSpace 只在建構子做
// 一次，不在 callGemini 裡，所以必須真的走 NewTransformUseCase 這條路；本檔其他
// 測試一律用 struct literal（&TransformUseCase{apiKey: ..., apiURL: ...}）或
// newTestTransformUseCase 直接建構，兩者都繞過建構子、驗證不到這行。
//
// pool 傳 nil：NewTransformUseCase 只把 pool 轉手存進 database.NewTxManager（不會
// 立即對它做任何操作，見 pkg/database/tx_manager.go 的 NewTxManager），這支測試
// 只呼叫 callGemini（純 HTTP，不碰 DB／txManager），不會用到 pool。
func TestNewTransformUseCase_TrimsAPIKeyWhitespace(t *testing.T) {
	var gotAPIKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("x-goog-api-key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(respSuccess))
	}))
	t.Cleanup(server.Close)

	uc := NewTransformUseCase(nil, &config.Config{GeminiAPIKey: "test-key\n"})
	uc.apiURL = server.URL

	_, _, err := uc.callGemini(context.Background(), []byte("fake"), "image/png", "test prompt")
	require.NoError(t, err)
	require.Equal(t, "test-key", gotAPIKey, "constructor must strings.TrimSpace cfg.GeminiAPIKey before storing it")
}
