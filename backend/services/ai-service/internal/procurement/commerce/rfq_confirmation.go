package commerce

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/pricing"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

var ErrStalePricePreview = errors.New("procurement: pricePreviewVersion or payloadHash doesn't match current price preview")

// ErrBuyerNoteTooLong：buyerNote 超過 domain.MaxBuyerNoteLen。
//
// ⛔ 長度是唯一對它做的檢查——內容不解析、不比對、不由它推導任何品相等級
// （v6.0 Part V）。驗證器重用 domain.ValidateBuyerNote：同一個欄位名、同一個
// 上限、同一條「拒絕而不是靜默截斷」的規則，早在 legacy quote-batches 的買家
// 覆蓋欄位就定好了（設計書 2026-08-15-procurement-quote-batches-buyer-overrides
// §5）。這一刀只是把它從即將退場的 quote-batch 路徑搬到 publication 路徑。
var ErrBuyerNoteTooLong = errors.New("procurement: buyerNote exceeds the length limit")

// normalizedBuyerNote 把已驗證的補充說明收斂成 canonical 形式：空字串回 nil。
// nil 而不是指向空字串的指標是刻意的——omitempty 只對 nil 生效，沒填時整個鍵
// 不得出現在 preimage 裡，否則既有 payload_hash 全部會變。
func normalizedBuyerNote(validated string) *string {
	if validated == "" {
		return nil
	}
	return &validated
}

// confirmedRfqSnapshotTTL：confirmed snapshot 要撐到買家決定發布（#2450）
// 為止，比 price preview 的 15 分鐘長得多；issue/v6.0 doc 未明定精確數字，
// 24 小時是合理的起始值，之後有實際數據可再調整。
const confirmedRfqSnapshotTTL = 24 * time.Hour

type ConfirmRfqSnapshotInput struct {
	RequestID           int64
	BuyerID             identity.BuyerID
	ClientRequestID     string
	ExpectedRevisionID  int64
	PricePreviewID      int64
	PricePreviewVersion int64
	PayloadHash         string
	// BuyerNote 是買家自己打字、原文轉呈給供應商的補充說明。空字串＝沒有。
	// ⛔ 不是 slot、不經 LLM 抽取、不進候選搜尋關鍵字——2026-08-17 移除的 spec
	// 是那種東西，本欄刻意不是（backend #2476／website #498）。
	BuyerNote string
}

type ConfirmedRfqSnapshotResult struct {
	ConfirmedRfqSnapshotID int64
	PayloadHash            string
	// ExpiresAt 是 snapshot 自己的到期（docx V.7.5 把 expiry 列為 snapshot 固定
	// 欄位之一），不是 price preview 的。
	ExpiresAt time.Time
}

// confirmedRfqPayload 是白名單挑選的 seller-facing projection：只放已核准
// 對外的欄位，明確不放 lane markup、BudgetPerUnit（buyer-private）——
// design spec D2 修正（V.7.4）明文排除這些欄位，見
// docs/superpowers/specs/2026-08-22-procurement-rfq-price-preview-design.md。
// PricePreviewResult 本身就不帶 Slots.BudgetPerUnit／MarkupRateByLane，白名單
// struct 只挑選其中已核准欄位，兩層防線疊加。
type confirmedRfqPayload struct {
	QualityKnowledgeState string `json:"qualityKnowledgeState"`
	// BuyerNote 是這次擴大 preimage 唯一的例外，而它**不是**破壞性變更：
	// omitempty ＋ 沒填時為 nil ⇒ 序列化結果與本欄加入前逐位元相同，既有的
	// payload_hash 一個都不會改。這正是 §3.1 那次（一律出現的欄位）做不到的事。
	BuyerNote           *string                     `json:"buyerNote,omitempty"`
	BuyerRequestedRange pricing.BuyerRequestedRange `json:"buyerRequestedRange"`
	// 以下六組是詢價單本身的識別性內容。它們原本不在 preimage 內，後果是兩張
	// 品項、數量、配送城市全都不同的詢價單，只要 pricing mode 與價格區間相同，
	// payload_hash 就一模一樣——買家接受時拿它當 CAS 依據，證明不了賣家報價當時
	// 看到的是哪一張詢價單。決策見 64 號設計書 §3.1（選項 B：擴大 preimage）。
	//
	// 同時這也讓 canonical_payload_json 真的能單獨編譯出 seller-facing payload
	// （64 號 §1.2）：下游不必再回頭 join procurement_requests／revision 表，
	// 因為那條 join 不受 payload_hash 保護。
	//
	// ⛔ 白名單紀律不變，擴大範圍不等於把整包 slots 倒進來。刻意不收：
	//   - BudgetPerUnit：買家私有預算，既有紅線（design spec D2 §V.7.4）。
	//   - ItemResolution.Provenance：「誰決定的」稽核欄位，不是賣家報價要看的
	//     識別內容。
	//   - ExactMatchRequired：買家給候選比對用的匹配偏好，不是賣家報價時要看的
	//     需求內容。
	//
	// Packaging 則是刻意「超出」64 號 §3.1 選項 B 字面列舉的一欄。§3.1 的清單
	// 承自 §1.2 對缺口的描述（品項／數量／單位／配送地／交期），但包裝需求同樣
	// 是賣家報價一定要看的內容——「紙箱」與「網袋」是兩個不同的報價。把它留到
	// 之後再收，就是第二次 preimage 破壞性變更，而這次改動的整個論證正是「不要
	// 有第二次」。趁 capability flag 全 OFF、零真實買家的同一個窗口一併收進來。
	//
	// 報價截止（quote deadline）同樣不在這裡：它是 publication 的屬性
	// （64 號 §3.3），confirm 當下還不存在，沒有來源可凍結。
	Item           *string                   `json:"item,omitempty"`
	ItemResolution *confirmedRfqItemIdentity `json:"itemResolution,omitempty"`
	Quantity       *int                      `json:"quantity,omitempty"`
	Unit           *string                   `json:"unit,omitempty"`
	Packaging      *string                   `json:"packaging,omitempty"`
	DeliveryCity   *string                   `json:"deliveryCity,omitempty"`
	DeliveryDate   *string                   `json:"deliveryDate,omitempty"`
}

// confirmedRfqItemIdentity 是 domain.ItemResolution 的 seller-facing 白名單投影：
// 只留「這是哪一個品項」需要的欄位。不直接內嵌 domain.ItemResolution，理由跟
// confirmedRfqPayload 不直接內嵌 PricePreviewResult 一樣——那個型別帶著
// Provenance（稽核）與內部分類 ID，序列化進來就是把非 seller-facing 的資料
// 寫進賣家看得到的 payload。
type confirmedRfqItemIdentity struct {
	Kind  string `json:"kind"`
	Label string `json:"label,omitempty"`
}

// sellerFacingItemIdentity 把 slots 的解析結果收斂成上面那個白名單投影。
// resolution 為 nil（品項還沒解析完成）時回 nil——此時品項
// 身分由 Item 原話承載，不補一個空殼進 preimage。
func sellerFacingItemIdentity(resolution *domain.ItemResolution) *confirmedRfqItemIdentity {
	if resolution == nil {
		return nil
	}
	return &confirmedRfqItemIdentity{
		Kind: string(resolution.Kind), Label: resolution.Label,
	}
}

// PricePreviewReader.GetPricePreview 依 (id, buyerID) 查詢，不是只依 id——
// 防止跨買家猜 pricePreviewId（V.7.4／issue AC 的 wrong-owner 同 404 要求）。
// 真實實作預期由 repository.Repo.FindPricePreviewByIDAndBuyer（本 task Step 0
// 新增）包一層轉成 PricePreviewResult 提供，wiring 留給 Task 10（同 Task 6／
// Task 7 的既有分工：usecase task 只定義介面，gateway-facing task 才動
// module.go 的 fx.Provide 綁定）。
type PricePreviewReader interface {
	GetPricePreview(ctx context.Context, id int64, buyerID identity.BuyerID) (pricing.PricePreviewResult, error)
}

// pricePreviewReaderAdapter 是 PricePreviewReader 的真實實作（Task 10，補上
// 上面那則註解說的 wiring）。*repository.Repo 沒有直接滿足 PricePreviewReader
// 的方法——它只有回傳 repository.PricePreview 的 FindPricePreviewByIDAndBuyer，
// 不像 PricePreviewRepo／RevisionReader 那樣能在 module.go 用一行轉型 closure
// 直接把 *repository.Repo 當介面注入，所以需要這一層轉接。轉換邏輯重用
// pricing/price_preview.go 既有的 pricing.ToPricePreviewResult（同一個
// repository.PricePreview → PricePreviewResult 轉換，PreviewRfqPriceUseCase
// 冪等命中時也是呼叫它），不重新發明第二套。
type pricePreviewReaderAdapter struct {
	repo *repository.Repo
}

// NewPricePreviewReader 建構 PricePreviewReader 的真實實作，供
// procurement/module.go 的 fx.Provide 使用。
func NewPricePreviewReader(repo *repository.Repo) PricePreviewReader {
	return pricePreviewReaderAdapter{repo: repo}
}

func (a pricePreviewReaderAdapter) GetPricePreview(ctx context.Context, id int64, buyerID identity.BuyerID) (pricing.PricePreviewResult, error) {
	row, err := a.repo.FindPricePreviewByIDAndBuyer(ctx, id, buyerID)
	if err != nil {
		return pricing.PricePreviewResult{}, err
	}
	return pricing.ToPricePreviewResult(row), nil
}

// ConfirmedRfqSnapshotRepo 的方法簽章逐字對齊 Task 8 的 repository.Repo，
// 跟 Task 6 的 PricePreviewRepo 是同一種既有慣例（usecase 直接依賴
// repository 套件型別，不重新發明一組平行型別）。
type ConfirmedRfqSnapshotRepo interface {
	InsertConfirmedRfqSnapshot(ctx context.Context, in repository.InsertConfirmedRfqSnapshotInput) (repository.ConfirmedRfqSnapshot, error)
	FindConfirmedRfqSnapshotByIdempotencyKey(ctx context.Context, buyerID identity.BuyerID, clientRequestID string) (repository.ConfirmedRfqSnapshot, error)
}

type ConfirmRfqSnapshotUseCase struct {
	repo          ConfirmedRfqSnapshotRepo
	previewReader PricePreviewReader
	revisions     pricing.RevisionReader
	// now 同 answer.go AnswerUseCase.now 的既有慣例：預設 time.Now().UTC()，
	// 測試可直接覆寫這個欄位（同套件內可見）來注入固定時間，驗證 TTL 過期
	// 判斷，不用 sleep 或本機 timezone 製造不穩定測試。
	now func() time.Time
}

func NewConfirmRfqSnapshotUseCase(repo ConfirmedRfqSnapshotRepo, previewReader PricePreviewReader, revisions pricing.RevisionReader) *ConfirmRfqSnapshotUseCase {
	return &ConfirmRfqSnapshotUseCase{
		repo: repo, previewReader: previewReader, revisions: revisions,
		now: func() time.Time { return time.Now().UTC() },
	}
}

func (uc *ConfirmRfqSnapshotUseCase) ConfirmRfqSnapshot(ctx context.Context, in ConfirmRfqSnapshotInput) (ConfirmedRfqSnapshotResult, error) {
	buyerNote, noteErr := domain.ValidateBuyerNote(in.BuyerNote)
	if noteErr != nil {
		return ConfirmedRfqSnapshotResult{}, fmt.Errorf("%w: %w", ErrBuyerNoteTooLong, noteErr)
	}
	// 冪等短路（同 Task 6 的兩層設計：這裡是效能優化，Task 8 的 DB unique
	// index 是最終正確性保證）。
	//
	// Task 17：命中不能無條件回放——跟 Task 15／I3 在 PreviewRfqPriceUseCase
	// 修過的同一種 bug，這裡是 ConfirmRfqSnapshotUseCase 的版本。
	// (buyerID, clientRequestID) 命中只證明「這個 key 之前用過」，不證明
	// 「這次呼叫的輸入」跟「當初算出 existing 的輸入」相同。
	//
	// 這張表沒有 PreviewRfqPrice 那種可重用的 PreviewInputHash 欄位（Will
	// 明確決定：zero-migration，不為此新增 DB column），改比對 existing 本來
	// 就持久化的三個欄位——RequestID／PricePreviewID／PricePreviewVersion——
	// 任一個對不上就代表買家（或有 bug 的 client）把同一個 clientRequestID
	// 用在了「另一個」確認請求上，屬於跟 turn.go／answer.go／rewind.go／
	// price_preview.go 同一種「clientRequestID 已用於其他請求」衝突，用同一個
	// ErrIdempotencyKeyConflict 哨兵錯誤，不是靜靜回放錯的那次結果。
	//
	// 刻意不比對 existing.PayloadHash：那是「已確認快照本身」的 payload hash
	// （從 confirmedRfqPayload 這個 seller-facing projection 算出來的），跟
	// in.PayloadHash（買家回傳、給下面 CAS 檢查用的「price preview」payload
	// hash）語意完全不同，比較兩者是 bug 不是修法——同一個 payload hash 名字
	// 在這個函式裡出現兩種不同語意，見下面 CAS 檢查段落對 in.PayloadHash 的
	// 使用方式。
	if existing, err := uc.repo.FindConfirmedRfqSnapshotByIdempotencyKey(ctx, in.BuyerID, in.ClientRequestID); err == nil {
		if existing.RequestID != in.RequestID || existing.PricePreviewID != in.PricePreviewID || existing.PricePreviewVersion != in.PricePreviewVersion {
			return ConfirmedRfqSnapshotResult{}, domain.ErrIdempotencyKeyConflict
		}
		return ConfirmedRfqSnapshotResult{ConfirmedRfqSnapshotID: existing.ID, PayloadHash: existing.PayloadHash, ExpiresAt: existing.ExpiresAt}, nil
	} else if !errors.Is(err, repository.ErrConfirmedRfqSnapshotNotFound) {
		return ConfirmedRfqSnapshotResult{}, err
	}

	// RevisionReader 沒有 GetActiveRevisionID（Task 6 修正期間已確認那個方法
	// 在 repository.Repo 裡不存在、已被移除——見 commit
	// 599d45091「drop dead GetActiveRevisionID」）。真實的 active revision
	// 來源是 GetSnapshot 回傳的 repository.Revision.ID，跟 Task 6 的
	// PreviewRfqPrice 用同一支方法、同一種讀法。
	_, rev, err := uc.revisions.GetSnapshot(ctx, in.RequestID, in.BuyerID)
	if err != nil {
		return ConfirmedRfqSnapshotResult{}, err
	}
	if rev.ID != in.ExpectedRevisionID {
		return ConfirmedRfqSnapshotResult{}, pricing.ErrStalePricePreviewRevision
	}
	activeRevisionID := rev.ID

	// GetPricePreview 依 (id, buyerID) 查——wrong-owner／random ID 在這裡
	// 就會被 repository 層擋下（repository.ErrPricePreviewNotFound），
	// 直接把底層錯誤往外傳，跟真的不存在同一種 404 shape。
	preview, err := uc.previewReader.GetPricePreview(ctx, in.PricePreviewID, in.BuyerID)
	if err != nil {
		return ConfirmedRfqSnapshotResult{}, err
	}
	// 三個 CAS guard（Task 14／I1）：Task 9 原本只比對 preview.Version／
	// PayloadHash，但兩者在 InsertPricePreview 當下就被寫死（Version 固定
	// 1，PayloadHash 只算一次、沒有 UPDATE 路徑），實際上只能抓到「client
	// 端自己傳錯 version/hash」，抓不到任何真正的過期／不一致。下面三個
	// guard 才是真正能偵測 staleness 的檢查，順序由「identity 錯誤」到
	// 「時間相關的 staleness」排列：
	//
	//   1. cross-request reuse：preview.RequestID 跟這次要 confirm 的
	//      in.RequestID 不同——FindPricePreviewByIDAndBuyer 刻意只用
	//      (id, buyerID) 查，不收斂到 request（wrong-owner 404 的既有設計，
	//      見該方法 doc comment），所以「這個 preview 屬於同一個買家的另一
	//      個 request」不會被 GetPricePreview 本身擋下，必須在這裡補上。
	//      Reviewer 修正（Task 14 fix round）：這裡刻意不沿用 wrong-owner
	//      的 404／RESOURCE_NOT_FOUND——wrong-owner 404 的隱私理由（不洩漏
	//      「這個 ID 存在，只是不是你的」給不同買家）在這裡不成立，因為
	//      preview 跟 confirm 是同一個買家，沒有跨買家隱私問題。實務上會
	//      觸發這裡的情境是 client 端狀態問題（分頁沒關的舊分頁、本地
	//      cache 存了舊 request 的 preview ID），跟下面第 2 點
	//      revision-changed-after-preview 同一種成因、同一種正確復原方式
	//      （回到現在這個 request 重新 PreviewRfqPrice，再 confirm），因此
	//      沿用同一個 ErrStalePricePreviewRevision／Aborted／STALE_REVISION
	//      shape，讓 client 只需要一種「stale，重新 preview 再試」的復原
	//      分支，而不是被迫區分「404 死路」與「409 重試」兩種語意。
	//   2. revision-changed-after-preview：preview 是針對
	//      preview.RequestRevisionID 這個 revision 算出來的，
	//      activeRevisionID 是 confirm 當下的現行 revision（上面已經驗證
	//      rev.ID == in.ExpectedRevisionID，但那只證明「client 回報的
	//      revision」跟「目前現行 revision」一致，不能證明「preview 本身」
	//      是針對現行 revision 算的——買家可能在 revision 5 時 preview，
	//      revision 之後被改成 6，買家帶著 expectedRevisionId=6（符合現況）
	//      卻拿著針對 revision 5 算出的 preview 來 confirm，這裡就是抓這個
	//      情境）。沿用既有 ErrStalePricePreviewRevision（已註冊
	//      Aborted／STALE_REVISION，confirmRfqSnapshotErrorStatus 的 doc
	//      comment 明講這類 revision/version/hash 不一致的 CAS 失敗共用同一
	//      個 reason code，不為每種細分情境新增 CodeMeta）。
	//   3. TTL 過期：preview.ExpiresAt 是 InsertPricePreview 當下算好、寫死
	//      的到期時間（pricePreviewTTL 常數）——price_preview.go 該常數的
	//      doc comment 原本就假設「過期的 preview 版本自然被拒」，但過去
	//      沒有任何程式碼真的讀 ExpiresAt 來擋，這裡補上。沿用
	//      ErrStalePricePreview（其中文文案「報價預覽已過期，請重新試算」
	//      正是 TTL 過期的字面描述，比 revision 那組更貼切）。
	//
	// 最後才是原本就有的 Version／PayloadHash echo 檢查：這組只能抓
	// client 自己傳錯值，訊號強度最弱，排在最後。
	if preview.RequestID != in.RequestID {
		return ConfirmedRfqSnapshotResult{}, pricing.ErrStalePricePreviewRevision
	}
	if preview.RequestRevisionID != activeRevisionID {
		return ConfirmedRfqSnapshotResult{}, pricing.ErrStalePricePreviewRevision
	}
	if uc.now().After(preview.ExpiresAt) {
		return ConfirmedRfqSnapshotResult{}, ErrStalePricePreview
	}
	if preview.Version != in.PricePreviewVersion || preview.PayloadHash != in.PayloadHash {
		return ConfirmedRfqSnapshotResult{}, ErrStalePricePreview
	}

	// slots 取自上面那一次 GetSnapshot 回來的 rev，不另外呼叫 GetSlots：
	// GetSlots 內部會再做一次 GetSnapshot，兩次讀之間 revision 可能換版，於是
	// 寫出「內容取自新版、request_revision_id 記的是舊版」的快照——正是這條線
	// 要消滅的那種不一致。rev 是剛剛通過 CAS 的那一個版本，用它才對得起
	// 下面 InsertConfirmedRfqSnapshot 記的 activeRevisionID。
	//
	// slots_json 是 NOT NULL DEFAULT '{}'（db/schema/procurement.sql），所以
	// 這裡直接嚴格反序列化，不為「空值」補一條寬鬆路徑——真的解不開就是資料
	// 壞了，該讓它出錯而不是靜靜凍結一份空內容。
	var slots domain.Slots
	if err := json.Unmarshal(rev.SlotsJSON, &slots); err != nil {
		return ConfirmedRfqSnapshotResult{}, err
	}

	payload := confirmedRfqPayload{
		QualityKnowledgeState: "BUYER_QUALITY_UNKNOWN",
		BuyerRequestedRange:   preview.BuyerRequestedRange,
		Item:                  slots.Item,
		ItemResolution:        sellerFacingItemIdentity(slots.ItemResolution),
		Quantity:              slots.Quantity,
		Unit:                  slots.Unit,
		Packaging:             slots.Packaging,
		DeliveryCity:          slots.DeliveryCity,
		DeliveryDate:          slots.DeliveryDate,
		BuyerNote:             normalizedBuyerNote(buyerNote),
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return ConfirmedRfqSnapshotResult{}, err
	}
	sum := sha256.Sum256(payloadBytes)
	payloadHash := hex.EncodeToString(sum[:])

	inserted, err := uc.repo.InsertConfirmedRfqSnapshot(ctx, repository.InsertConfirmedRfqSnapshotInput{
		BuyerID: in.BuyerID, RequestID: in.RequestID, RequestRevisionID: activeRevisionID,
		PricePreviewID: in.PricePreviewID, PricePreviewVersion: in.PricePreviewVersion,
		QualityKnowledgeState: "BUYER_QUALITY_UNKNOWN",
		CanonicalPayloadJSON:  payloadBytes, PayloadHash: payloadHash,
		ClientRequestID: in.ClientRequestID, ExpiresAt: time.Now().Add(confirmedRfqSnapshotTTL),
	})
	if err != nil {
		// Task 18／Fix 7：repository.InsertConfirmedRfqSnapshot 在真正併發撞鍵、
		// 且撞鍵後查到的既有列跟這次輸入不一致時，回這個哨兵——映射成
		// ErrIdempotencyKeyConflict，跟上面應用層短路檢查（第 135-142 行）撞到
		// RequestID／PricePreviewID／PricePreviewVersion 不一致時同一種對外語意，
		// 只是這裡是併發競態的最終仲裁路徑。
		if errors.Is(err, repository.ErrConfirmedRfqSnapshotIdempotencyConflict) {
			return ConfirmedRfqSnapshotResult{}, domain.ErrIdempotencyKeyConflict
		}
		return ConfirmedRfqSnapshotResult{}, err
	}
	return ConfirmedRfqSnapshotResult{ConfirmedRfqSnapshotID: inserted.ID, PayloadHash: inserted.PayloadHash, ExpiresAt: inserted.ExpiresAt}, nil
}
