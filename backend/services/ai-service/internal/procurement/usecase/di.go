package usecase

import (
	"go.uber.org/fx"
)

// Module 只提供 usecase 層自己的建構子。NewTurnUseCase 的 fx.Provide 已由 T12
// procurement-graph-cutover 計畫 Task 6a 移除：TurnUseCase／Handle／HandleStream／
// ClaimAndDispatchTurn／executeTurnPipeline 已整批刪除，SubmitTurn／
// SubmitTurnStream 兩個 gRPC 入口改為無條件走 graph（ClaimAndDispatchTurnViaGraph，
// graph_dispatch.go）。
//
// SlotExtractor 介面雖然宣告在本 package，但它的建構子（usecase.NewSlotExtractor，
// 依 EXTRACTOR_MODEL 選出該用哪個 provider 實作、直接回傳 SlotExtractor 介面）
// 仍放在 procurement/module.go 提供，不放這裡——理由是一致性而非技術限制：
// module.go 是所有介面綁定（誰實作了誰宣告的介面）統一收攏的組合根（見該檔案
// 開頭註解），RevisionStore／SnapshotReader 這兩個綁定都在那裡；NewSlotExtractor
// 雖然不像它們一樣需要另外包一層轉換函式，仍跟著放在一起，讓「介面綁定一律去
// module.go 找」這條規則沒有例外。
var Module = fx.Module("procurement_usecase",
	fx.Provide(NewAnswerUseCase),
	fx.Provide(NewRewindUseCase),
)
