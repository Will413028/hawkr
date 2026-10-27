package grpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/actor"
	"github.com/Will413028/hawkr/ai-service/internal/config"
	"github.com/Will413028/hawkr/ai-service/internal/usecase"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pb "github.com/Will413028/hawkr/go-pkg/proto/ai"
)

// 本檔驗證 #3519 slice B Task 4：13 處手刻 bizcode envelope 不得再把
// err.Error() 原封不動送給呼叫端——分類為「未歸類 fallback」的那幾處必須換成
// genericFailureMessage，分類為 DomainError 的那幾處必須改走 domainMessage()
// （即 DomainMessage()，不是 Error()）。task-4-classification.md 是本檔測試對應的
// 判定表。

// --- domainMessage() 本身：兩個方向都要證明 ---

// 方向一（本次修復要防的洩漏）：DomainError 包了下游 cause 時，domainMessage
// 必須只回 Message，不能把 Cause 一起帶出去——這正是 err.Error() 和
// DomainMessage() 唯一的差異，也是 13 處手刻 envelope 繞過
// error_interceptor.go 造成洩漏的根因。沒有這支測試，"服務暫時無法使用" 的
// genericFailureMessage 分支測試不出來 domainMessage 本身在「已分類」分支到底
// 有沒有把 Cause 濾掉。
func TestDomainMessage_WrappedDomainError_DropsCause(t *testing.T) {
	cause := errors.New("sentinel-cause-7b1c")
	err := pkgerrors.Wrap(pkgerrors.CodeInvalidInput, "額度不足", cause)

	// 對照組：Error() 本來就會把 cause 串進去，用來證明兩者行為真的不同，
	// 不是巧合地兩個都不含 sentinel。
	if !strings.Contains(err.Error(), "sentinel-cause-7b1c") {
		t.Fatalf("test setup invalid: DomainError.Error() should still contain the cause, got %q", err.Error())
	}

	got := domainMessage(err)
	if got != "額度不足" {
		t.Errorf("domainMessage() = %q, want %q", got, "額度不足")
	}
	if strings.Contains(got, "sentinel-cause-7b1c") {
		t.Errorf("domainMessage() leaked the wrapped cause: %q", got)
	}
}

// 方向二：err 完全不是 DomainCoder（型別斷言失敗）時，domainMessage 必須回退
// 固定文案，不能掉回 err.Error()。
func TestDomainMessage_NonDomainError_FallsBackToGenericMessage(t *testing.T) {
	err := errors.New("sentinel-non-domain-4c2a")

	got := domainMessage(err)
	if got != genericFailureMessage {
		t.Errorf("domainMessage() = %q, want fixed message %q", got, genericFailureMessage)
	}
	if strings.Contains(got, "sentinel-non-domain-4c2a") {
		t.Errorf("domainMessage() leaked the original error text: %q", got)
	}
}

// --- fix round 1（controller 回饋）：deliberate 業務訊息不得被 fallback 吃掉 ---
//
// quota_usecase.go／style_usecase.go 的幾個「自己寫的、不含系統細節」的驗證
// 訊息（weeklyLimit 範圍、sort order 範圍、style not found、cannot delete
// default style）已改成 pkgerrors.NewBusiness(400, "<原文>")，讓
// pkgerrors.GetCode 判為已分類、domainMessage() 原樣透傳（NewBusiness 不帶
// Cause，Message 與修復前的 err.Error() 逐字元相同）。下面兩支測試證明「業務
// 訊息透傳」這個方向；usecase 層對「真正 DB 錯誤仍不被誤判成業務錯誤」的驗證見
// internal/usecase/style_usecase_domain_error_test.go、
// internal/usecase/quota_usecase_domain_error_test.go（用 gen.DBTX 造假，不需要
// 真實 Postgres）。
//
// ⚠️ 這次轉換的副作用：UpdateAiStyle／UpdateAiQuota 原本「不需要真實 DB 就能
// 觸發 fallback」的唯一入口（sort order 溢位、weeklyLimit 越界）現在都變成
// classified 業務錯誤了，不再落到 fallback。因此在本檔（grpc package，
// styleUseCase／quotaUseCase 是具體型別而非介面，無法從外部注入假 DBTX）已經
// 沒有不動用 dbtest／docker 就能觸發「真正未歸類 DB 錯誤」的手段——這個方向的
// handler 端到端驗證，現在跟 DeleteAiStyle／DeleteAiQuota 的 fallback 一樣，
// 只能在 usecase 層證明（已完成，見上述兩個測試檔），或需要 dbtest／docker 才能
// 在 handler 層驗證。TransformImage 的 fallback 是例外：apiKey 留空的
// short-circuit 完全沒動過，仍可在不碰 DB 的情況下於 handler 層直接驗證，見下方
// TestTransformImage_UnclassifiedErrorHidesDetail。

// UpdateAiStyle 的 SortOrder 溢位（handler.go 的 classified 分支，經
// domainMessage()）：業務訊息必須逐字元透傳，不能被固定文案取代。
func TestUpdateAiStyle_SortOrderOutOfRange_ReturnsExactBusinessMessage(t *testing.T) {
	s := &AIGrpcServer{styleUseCase: usecase.NewStyleUseCase(nil)}

	resp, err := s.UpdateAiStyle(context.Background(), &pb.UpdateAiStyleRequest{
		Id:           "123",
		HasSortOrder: true,
		SortOrder:    "99999999999", // > math.MaxInt32，parseSortOrder 判定為 out of range
	})
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if resp.Code != 400 {
		t.Fatalf("Code = %d, want 400", resp.Code)
	}
	want := "sort order out of range: 99999999999"
	if resp.Message != want {
		t.Errorf("Message = %q, want exact business message %q", resp.Message, want)
	}
	if resp.Message == genericFailureMessage {
		t.Errorf("business message was swallowed into the generic fallback")
	}
}

// UpdateAiQuota 的 weeklyLimit 越界（handler.go 的 classified 分支，經
// domainMessage()）：業務訊息必須逐字元透傳，不能被固定文案取代——這正是
// controller fix round 1 指名要修的 UX 倒退。
func TestUpdateAiQuota_WeeklyLimitOutOfRange_ReturnsExactBusinessMessage(t *testing.T) {
	s := &AIGrpcServer{quotaUseCase: usecase.NewQuotaUseCase(nil)}

	resp, err := s.UpdateAiQuota(context.Background(), &pb.UpdateAiQuotaRequest{
		StoreId:     "1",
		WeeklyLimit: "9999", // 超出 0~1000，quotaUseCase.UpdateQuota 在碰 DB 前就回傳 NewBusiness(400, ...)
	})
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if resp.Code != 400 {
		t.Fatalf("Code = %d, want 400", resp.Code)
	}
	want := "weeklyLimit 必須為 0~1000 的整數"
	if resp.Message != want {
		t.Errorf("Message = %q, want exact business message %q", resp.Message, want)
	}
	if resp.Message == genericFailureMessage {
		t.Errorf("business message was swallowed into the generic fallback")
	}
}

// TransformImage 的 else fallback（handler.go:501，票原本點名的那一處）：
// apiKey 留空時 TransformUseCase.Transform 在呼叫 Gemini／碰 DB 之前就以
// fmt.Errorf("AI service not configured") short-circuit，不需要真實 pool 或網路。
func TestTransformImage_UnclassifiedErrorHidesDetail(t *testing.T) {
	s := &AIGrpcServer{
		transformUseCase: usecase.NewTransformUseCase(nil, &config.Config{}), // GeminiAPIKey 留空
	}
	ctx := context.WithValue(context.Background(), actorContextKey{}, actor.NewStoreActor(identity.StoreID(1)))

	resp, err := s.TransformImage(ctx, &pb.TransformImageRequest{
		ImageData: []byte("fake-image-bytes"),
		MimeType:  "image/png",
		Prompt:    "make it nicer",
	})
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if resp.Code != 500 {
		t.Fatalf("Code = %d, want 500", resp.Code)
	}
	if strings.Contains(resp.Message, "AI service not configured") {
		t.Errorf("response leaked usecase error detail: %q", resp.Message)
	}
	if resp.Message != genericFailureMessage {
		t.Errorf("Message = %q, want fixed message %q", resp.Message, genericFailureMessage)
	}
}
