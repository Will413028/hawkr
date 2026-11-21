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

// B1-1 只有 REST driver（對舊的 media-service）：Connect driver 與 monolith_integration_test.go 在 B1-3 隨新契約補上，
// 屆時同一份情境對單體再跑一次。這裡只涵蓋不需要物件儲存的兩個端點（B1-D12）：機敏上傳與機敏預覽在 CI 的 Java
// 沒有 R2 設定時測不到，行為規格在 docs/plans/2026-11-21-b1-media/test-map.md。

import (
	"context"

	ch "github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
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
}

// Driver 是情境操作 media 的方式；錯誤對應到 characterize 的分類。
type Driver interface {
	CreateUploadSession(ctx context.Context, c ch.Caller, in UploadInput) (UploadSession, error)
	CompleteUploadSession(ctx context.Context, c ch.Caller, sessionID string) (CompletedSession, error)
}
