// Package characterization 是 media 對外 API 的特性測試（ADR 2026-09-24-go-modular-monolith D8 的第一層
// 安全網、ADR 2026-09-26-external-contract-protobuf-connect D4），共用的部分在 internal/httpapi/characterize。
//
// 情境只寫一次（scenarios_test.go），透過 Driver 操作：REST driver 經 gateway 打舊的 media-service、Connect driver 打單體。
// 兩種執行：
//   - 直接打單體（monolith_integration_test.go，build tag integration，CI 跑），並以突變確認情境抓得到錯的資料
//     （照 content 的 TestScenariosCatchWrongData 寫）。
//   - 經本機的 gateway（gateway_test.go，build tag gateway，scripts/characterize.sh media 起服務後執行）。
//
// 兩個 driver 的 wire 不同，情境只看業務上的結果：ID 當成不透明的字串、錯誤只比 characterize 的分類。與舊實作
// 刻意的差異（模組 CARRYOVER.md「與舊服務的差異」）不寫成共用的情境。media-service 刪除時，REST driver、legacy.sh
// 與 TestLegacy 一起刪除。
package characterization

// 兩個 driver：REST（經舊 gateway 對舊的 media-service，rest_test.go）與 Connect（對單體的 MediaAppService，connect_test.go）。
// 共用的情境只涵蓋不需要物件儲存的兩個端點（B1-D12）：機敏上傳與機敏預覽在 CI 的 Java 沒有 R2 設定時測不到，行為規格在
// backend/docs/plans/2026-11-21-b1-media/test-map.md，由只有單體 driver 能跑的情境（monolith_scenarios_test.go）依它覆蓋。

import (
	"context"

	ch "github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
)

// Variant 標示 driver 打的是哪個實作。與舊實作刻意不同的情境依它分支，分支旁寫決策代號與新行為，不刪情境。
type Variant int

const (
	// Legacy 是舊的 Java media-service。
	Legacy Variant = iota
	// Monolith 是單體的 Go media 模組（MediaAppService 與機敏段上傳端點）。
	Monolith
)

// UploadInput 是建立上傳會話的宣告。
type UploadInput struct {
	Filename    string
	ContentType string
	// FileSize 是宣告的位元組數，wire 上是十進位字串。
	FileSize string
	IsSecure bool
}

// UploadSession 是建立上傳會話的結果。
type UploadSession struct {
	ID string
	// UploadURL 只有公開上傳有值。
	UploadURL string
	// ExpiresIn 是會話存活秒數（十進位字串）。
	ExpiresIn string
	IsSecure  bool
}

// CompletedSession 是完成上傳會話的結果。
type CompletedSession struct {
	StagedToken string
	PreviewURL  string
	// State 是完成之後讀到的暫存狀態；只有 Connect driver 有（GetUpload），舊服務沒有這個讀取。
	State string
}

// Driver 是情境操作 media 的方式；錯誤對應到 characterize 的分類。
type Driver interface {
	Variant() Variant
	CreateUploadSession(ctx context.Context, c ch.Caller, in UploadInput) (UploadSession, error)
	CompleteUploadSession(ctx context.Context, c ch.Caller, sessionID string) (CompletedSession, error)
}

// SecureSession 是機敏分段上傳會話。
type SecureSession struct {
	Token      string
	ChunkSize  int
	ChunkCount int
}

// MonolithDriver 是只有單體實作的操作：機敏分段上傳（B1-D17），舊的 Java 在 CI 沒有物件儲存，對它測不到（B1-D12）。
type MonolithDriver interface {
	Driver
	BeginSecureUpload(ctx context.Context, c ch.Caller, size int64) (SecureSession, error)
	// PutSecureChunk 送出一段並回 HTTP 狀態碼；錯誤只代表傳輸失敗，業務結果看狀態碼。
	PutSecureChunk(ctx context.Context, c ch.Caller, token string, index int, data []byte) (int, error)
}
