package commerce

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/ai-service/internal/usecase"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"go.uber.org/zap"
)

var (
	// ErrStaleRfqSnapshot 對應 STALE_RFQ_SNAPSHOT。
	ErrStaleRfqSnapshot = errors.New("procurement: rfq snapshot changed since it was read")

	// ErrRfqSnapshotHashRequired 對應 §11 的 invalid argument：client 沒送 CAS token。
	// ⛔ 不得由 server 以「目前的權威值」補上——那等於把 doc 64 §3.2 的兩個 CAS 對象
	// 縮成一個，而少的那一個正好是「詢價單被換掉」那條錯配路徑。
	ErrRfqSnapshotHashRequired = errors.New("procurement: expectedRfqSnapshotHash is required")

	// ErrRfqResponseExpired 對應 RFQ_RESPONSE_EXPIRED。
	ErrRfqResponseExpired = errors.New("procurement: seller response version has passed validUntil")

	// ErrRfqRecipientClosed 對應 RFQ_RECIPIENT_CLOSED。
	ErrRfqRecipientClosed = errors.New("procurement: seller response write is closed")

	// ErrRfqCheckoutAlreadyActive 對應 RFQ_CHECKOUT_ALREADY_ACTIVE：這張 request
	// 已經有一筆 ACTIVE acceptance，買家該去付款而不是重新接受。
	ErrRfqCheckoutAlreadyActive = errors.New("procurement: request already has an active acceptance")

	// ErrRfqAlreadyAwarded 對應 RFQ_ALREADY_AWARDED：已有 payment-finalized winner。
	// 這是唯一的硬封鎖點（wire 契約 §5.4-4）。
	ErrRfqAlreadyAwarded = errors.New("procurement: request already has a payment-finalized award")
)

// BuyerAcceptanceStore 是四支買家命令需要的持久層能力。
type BuyerAcceptanceStore interface {
	ListSellerResponseVersionsForRequest(ctx context.Context, buyerID identity.BuyerID, requestID int64) ([]repository.SellerResponseVersionWithIdentity, error)
	FindSellerResponseByID(ctx context.Context, id int64) (repository.SellerResponseIdentity, error)
	// ⛔ 只能在 transaction context 內呼叫：acceptance 的 CAS 讀要鎖住 identity 列，
	// 否則賣家改版／撤回可以在讀與寫之間 commit。
	FindSellerResponseByIDForUpdate(ctx context.Context, id int64) (repository.SellerResponseIdentity, error)
	FindSellerResponseVersion(ctx context.Context, sellerResponseID int64, version int32) (repository.SellerResponseVersion, error)
	FindPublicationByID(ctx context.Context, id int64) (repository.Publication, error)
	FindConfirmedRfqSnapshotByIDAndBuyer(ctx context.Context, id int64, buyerID identity.BuyerID) (repository.ConfirmedRfqSnapshot, error)

	PutShortlistEntry(ctx context.Context, buyerID identity.BuyerID, requestID, sellerResponseID int64, sellerResponseVersion int32) (repository.BuyerShortlistEntry, error)
	DeleteShortlistEntry(ctx context.Context, buyerID identity.BuyerID, requestID, sellerResponseID int64) (bool, error)
	ListShortlistEntries(ctx context.Context, buyerID identity.BuyerID, requestID int64) ([]repository.BuyerShortlistEntry, error)

	InsertBuyerQuoteAcceptance(ctx context.Context, in repository.InsertBuyerQuoteAcceptanceInput) (repository.BuyerQuoteAcceptance, error)
	FindBuyerQuoteAcceptanceByIdempotencyKey(ctx context.Context, buyerID identity.BuyerID, clientRequestID string) (repository.BuyerQuoteAcceptance, error)
	FindLiveBuyerQuoteAcceptanceForRequest(ctx context.Context, buyerID identity.BuyerID, requestID int64) (repository.BuyerQuoteAcceptance, error)
	InsertOrderProvisioningOutbox(ctx context.Context, acceptanceID int64, payload json.RawMessage, now time.Time) error
}

type BuyerAcceptanceUseCase struct {
	store  BuyerAcceptanceStore
	tx     TxRunner
	seller usecase.SellerClient
	now    func() time.Time
}

func NewBuyerAcceptanceUseCase(
	store BuyerAcceptanceStore, tx TxRunner, seller usecase.SellerClient, now func() time.Time,
) *BuyerAcceptanceUseCase {
	if now == nil {
		now = time.Now
	}
	return &BuyerAcceptanceUseCase{store: store, tx: tx, seller: seller, now: now}
}

// OrderShellRequestedPayload 是建單命令的 JSONB 內容。
//
// ⚠️ 它是 producer（本 usecase）與 consumer（orderprovisioning worker）之間的契約，
// 所以導出而不是各留一份 struct——兩份會悄悄漂開，而漂開的徵兆只有「worker 解不出
// 欄位」這種上線後才看得到的形態。
//
// ⛔ 只帶識別，不重送內容——與 doc 64 §9.1 對命令的判準一致。acceptance 與被接受的
// seller response version 都是 immutable，worker 照 id 讀回來的必然是同一份事實；把金額
// 與品名複製一份進 payload 只會多出一份會漂移的真相。ID 一律字串（避免 JS 精度問題）。
type OrderShellRequestedPayload struct {
	SchemaVersion int    `json:"schemaVersion"`
	AcceptanceID  string `json:"acceptanceId"`
	BuyerID       string `json:"buyerId"`
	RequestID     string `json:"requestId"`
}

var (
	PartyTypePlatformStore  = codemeta.ProcurementSellerPartyTypePlatformStore.Value
	PartyTypeExternalVendor = codemeta.ProcurementSellerPartyTypeExternalVendor.Value
)

// SellerIdentityView 是可公開的賣家身分投影。
// ⛔ 不得帶站外業者 email、KYC 狀態或 prospect 聯絡方式。
//
// EXTERNAL lane 的 DisplayName 目前恆為空：站外業者的顯示名住 seller-service 的
// seller_prospects，ai-service 這一側沒有安全可用的快照。PartyType 仍然權威，UI 據它
// 渲染「站外業者」而不是編一個名字。
type SellerIdentityView struct {
	DisplayName string
	PartyType   string
	StoreCode   string
}

// BuyerFacingSellerResponse 是比較清單的一列：一個 immutable version。
type BuyerFacingSellerResponse struct {
	Identity      repository.SellerResponseIdentity
	Version       repository.SellerResponseVersion
	Status        string
	Acceptability string
	Shortlisted   bool
	Seller        SellerIdentityView
}

// ListSellerResponses 回這張 request 底下所有賣家的所有 immutable 版本。
// 舊版本刻意留在清單裡（V.11.5：留在 audit 但不可接受），可否接受由 typed
// acceptability 表達，前端不得自行推導。
func (uc *BuyerAcceptanceUseCase) ListSellerResponses(
	ctx context.Context, buyerID identity.BuyerID, requestID int64,
) ([]BuyerFacingSellerResponse, error) {
	rows, err := uc.store.ListSellerResponseVersionsForRequest(ctx, buyerID, requestID)
	if err != nil {
		return nil, err
	}
	shortlisted, err := uc.store.ListShortlistEntries(ctx, buyerID, requestID)
	if err != nil {
		return nil, err
	}
	// key 是 (sellerResponseID, version)：shortlist 的唯一鍵是 (request, response)，
	// 換版本等於改掛在新版本上，所以舊版本那一列不再是 shortlisted。
	type shortlistKey struct {
		responseID int64
		version    int32
	}
	inShortlist := make(map[shortlistKey]struct{}, len(shortlisted))
	for _, e := range shortlisted {
		inShortlist[shortlistKey{e.SellerResponseID, e.SellerResponseVersion}] = struct{}{}
	}

	now := uc.now().UTC()
	out := make([]BuyerFacingSellerResponse, 0, len(rows))
	for _, row := range rows {
		status := domain.SellerResponseVersionStatus(
			row.Version.Version, row.Identity.LatestVersion, row.Identity.WithdrawnVersion,
			row.Version.ValidUntil, now,
		)
		_, isShortlisted := inShortlist[shortlistKey{row.Version.SellerResponseID, row.Version.Version}]
		out = append(out, BuyerFacingSellerResponse{
			Identity:      row.Identity,
			Version:       row.Version,
			Status:        status,
			Acceptability: domain.QuoteAcceptability(status, row.Identity.WriteClosed()),
			Shortlisted:   isShortlisted,
			Seller:        SellerIdentityView{PartyType: partyTypeForLane(row.Identity.Lane)},
		})
	}
	uc.attachSellerIdentity(ctx, out)
	return out, nil
}

func partyTypeForLane(lane string) string {
	if lane == repository.SellerResponseLaneExternal {
		return PartyTypeExternalVendor
	}
	return PartyTypePlatformStore
}

// attachSellerIdentity 用一次 BatchGetSellerMeta 補齊 PLATFORM 賣家的顯示欄位
// （呼叫次數 O(1)）。失敗 fail-open，同 candidate_platform_lane.attachSellerMeta：
// 比較清單的阻塞內容是報價本身，名片退化成只有 partyType 仍然可用。
func (uc *BuyerAcceptanceUseCase) attachSellerIdentity(ctx context.Context, rows []BuyerFacingSellerResponse) {
	if uc.seller == nil || len(rows) == 0 {
		return
	}
	seen := map[identity.StoreID]struct{}{}
	storeIDs := make([]identity.StoreID, 0, len(rows))
	for _, r := range rows {
		if r.Identity.StoreID == nil {
			continue
		}
		id := identity.StoreID(*r.Identity.StoreID)
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		storeIDs = append(storeIDs, id)
	}
	if len(storeIDs) == 0 {
		return
	}
	metas, err := uc.seller.BatchGetSellerMeta(ctx, storeIDs)
	if err != nil {
		pkglogger.Warn(ctx, "procurement: BatchGetSellerMeta failed; seller responses keep partyType only",
			zap.Error(err), zap.Int("storeCount", len(storeIDs)))
		return
	}
	for i := range rows {
		if rows[i].Identity.StoreID == nil {
			continue
		}
		meta := metas[identity.StoreID(*rows[i].Identity.StoreID)]
		if meta == nil {
			continue
		}
		rows[i].Seller.DisplayName = meta.SellerName
		rows[i].Seller.StoreCode = meta.StoreCode
	}
}

// PutShortlist 加入比較。冪等、零 side effect，因此不需要冪等鍵。
func (uc *BuyerAcceptanceUseCase) PutShortlist(
	ctx context.Context, buyerID identity.BuyerID, requestID, sellerResponseID int64, version int32,
) (repository.BuyerShortlistEntry, error) {
	if _, err := uc.loadOwnedVersion(ctx, buyerID, requestID, sellerResponseID, version); err != nil {
		return repository.BuyerShortlistEntry{}, err
	}
	return uc.store.PutShortlistEntry(ctx, buyerID, requestID, sellerResponseID, version)
}

// DeleteShortlist 移除比較。冪等：本來就不在清單裡也回成功。
func (uc *BuyerAcceptanceUseCase) DeleteShortlist(
	ctx context.Context, buyerID identity.BuyerID, requestID, sellerResponseID int64,
) (bool, error) {
	return uc.store.DeleteShortlistEntry(ctx, buyerID, requestID, sellerResponseID)
}

// loadOwnedVersion 解析一個 exact version 並驗證它確實掛在這位買家的這張 request 上。
// 跨買家或跨 request 一律 ErrSellerResponseNotFound（→404），不回述請求的 id。
func (uc *BuyerAcceptanceUseCase) loadOwnedVersion(
	ctx context.Context, buyerID identity.BuyerID, requestID, sellerResponseID int64, version int32,
) (repository.SellerResponseVersion, error) {
	id, err := uc.store.FindSellerResponseByID(ctx, sellerResponseID)
	if err != nil {
		return repository.SellerResponseVersion{}, err
	}
	if id.BuyerID != buyerID || id.RequestID != requestID {
		return repository.SellerResponseVersion{}, repository.ErrSellerResponseNotFound
	}
	return uc.store.FindSellerResponseVersion(ctx, sellerResponseID, version)
}

type AcceptSellerQuoteInput struct {
	BuyerID                       identity.BuyerID
	RequestID                     int64
	ClientRequestID               string
	SellerResponseID              int64
	ExpectedSellerResponseVersion int32
	ExpectedRfqSnapshotHash       string
}

type AcceptSellerQuoteResult struct {
	Acceptance repository.BuyerQuoteAcceptance
	Replayed   bool
}

// AcceptSellerQuote 接受一個 exact version。Server 權威 ACK 即商務 award。
//
// ⛔ 請求不帶任何金額：acceptedDealPrice 由 server 從被接受的那一版取，client 送不
// 進來（wire 契約 §5.3）。
func (uc *BuyerAcceptanceUseCase) AcceptSellerQuote(
	ctx context.Context, in AcceptSellerQuoteInput,
) (AcceptSellerQuoteResult, error) {
	if in.ClientRequestID == "" {
		return AcceptSellerQuoteResult{}, ErrIdempotencyKeyRequired
	}
	// 必填驗證排在回放查找之前：缺 CAS token 是「這個命令本身不合法」，不是
	// 「這個命令做過沒有」，不能讓它靠一把重複的冪等鍵繞過。
	if in.ExpectedRfqSnapshotHash == "" {
		return AcceptSellerQuoteResult{}, ErrRfqSnapshotHashRequired
	}
	// ⛔ 回放查找排在所有 CAS 守門之前，理由同賣家 submit：賣家在兩次重試之間改版時，
	// 先跑 CAS 會回 STALE_SELLER_RESPONSE，但第一次其實已經成功、RFQ 已經 award。
	//
	// 但回放的前提是「同鍵**同內容**」。既有列自己就帶著當初被接受的完整意圖
	// （request／sellerResponse／exact version／RFQ hash），拿它跟這次的輸入逐欄比，
	// 不一致就是 doc 64 §9.2 的 IDEMPOTENCY_KEY_CONFLICT：零 mutation，且
	// **不得靜默回放上一次的結果**。比 request_hash 不夠——它存的是賣家報價版本的
	// hash，換一張 request 或換一份 snapshot 都不會讓它改變。
	if existing, findErr := uc.store.FindBuyerQuoteAcceptanceByIdempotencyKey(ctx, in.BuyerID, in.ClientRequestID); findErr == nil {
		if existing.RequestID != in.RequestID ||
			existing.SellerResponseID != in.SellerResponseID ||
			existing.SellerResponseVersion != in.ExpectedSellerResponseVersion ||
			existing.ConfirmedRfqPayloadHash != in.ExpectedRfqSnapshotHash {
			return AcceptSellerQuoteResult{}, repository.ErrBuyerQuoteAcceptanceIdempotencyConflict
		}
		return AcceptSellerQuoteResult{Acceptance: existing, Replayed: true}, nil
	} else if !errors.Is(findErr, repository.ErrBuyerQuoteAcceptanceNotFound) {
		return AcceptSellerQuoteResult{}, findErr
	}

	// 截到微秒：Postgres timestamptz 就是微秒精度，截過之後 RETURNING 回來的
	// accepted_at 對「這次真的插進去」的那一列會與這個值完全相等，成為可靠的回放判準。
	now := uc.now().UTC().Truncate(time.Microsecond)

	// ⛔ 所有 CAS 讀與 acceptance＋outbox 的寫入在同一個 transaction 內，且 identity 列
	// 先鎖住（doc 64 §2-3、§6.2 選項 B）。在交易外讀 latest_version／withdrawn_version，
	// 賣家的改版或撤回可以在「讀完」與「寫入」之間 commit，acceptance 就落在一個已被
	// supersede 的版本上——只把 SELECT 搬進 READ COMMITTED 交易而不加鎖並不能解決它。
	// 鎖序與賣家側 submit／withdraw 一致：都先取 procurement_seller_responses 那一列。
	var acceptance repository.BuyerQuoteAcceptance
	err := uc.tx.WithTx(ctx, func(txCtx context.Context) error {
		sellerResponse, findErr := uc.store.FindSellerResponseByIDForUpdate(txCtx, in.SellerResponseID)
		if findErr != nil {
			return findErr
		}
		if sellerResponse.BuyerID != in.BuyerID || sellerResponse.RequestID != in.RequestID {
			return repository.ErrSellerResponseNotFound
		}
		if sellerResponse.LatestVersion != in.ExpectedSellerResponseVersion {
			// 已改價或已撤回都走同一個碼：買家手上那一版不再是可接受的那一版，
			// **必須明確告知報價已變動**，不得靜默改接受最新版。
			return repository.ErrSellerResponseStale
		}
		if sellerResponse.WithdrawnVersion != nil && *sellerResponse.WithdrawnVersion == in.ExpectedSellerResponseVersion {
			return repository.ErrSellerResponseStale
		}
		if sellerResponse.WriteClosed() {
			return ErrRfqRecipientClosed
		}

		version, versionErr := uc.store.FindSellerResponseVersion(txCtx, in.SellerResponseID, in.ExpectedSellerResponseVersion)
		if versionErr != nil {
			return versionErr
		}
		if now.After(version.ValidUntil) {
			return ErrRfqResponseExpired
		}

		pub, pubErr := uc.store.FindPublicationByID(txCtx, sellerResponse.PublicationID)
		if pubErr != nil {
			return pubErr
		}
		snapshot, snapErr := uc.store.FindConfirmedRfqSnapshotByIDAndBuyer(txCtx, pub.ConfirmedRfqSnapshotID, in.BuyerID)
		if snapErr != nil {
			return snapErr
		}
		// 空字串已在入口擋掉，這裡不再留 `!= ""` 的短路——留著等於保留一條「沒送就不 CAS」
		// 的旁路，而那正是原本的缺陷。
		if in.ExpectedRfqSnapshotHash != snapshot.PayloadHash {
			return ErrStaleRfqSnapshot
		}

		inserted, insertErr := uc.store.InsertBuyerQuoteAcceptance(txCtx, repository.InsertBuyerQuoteAcceptanceInput{
			BuyerID:                   in.BuyerID,
			RequestID:                 in.RequestID,
			PublicationID:             pub.ID,
			SellerResponseID:          in.SellerResponseID,
			SellerResponseVersion:     version.Version,
			SellerResponsePayloadHash: version.PayloadHash,
			ConfirmedRfqSnapshotID:    snapshot.ID,
			ConfirmedRfqPayloadHash:   snapshot.PayloadHash,
			Lane:                      sellerResponse.Lane,
			StoreID:                   sellerResponse.StoreID,
			ProspectID:                sellerResponse.ProspectID,
			// acceptedDealPrice 的來源只有被接受的那一版。
			AcceptedDealPrice: version.TotalAmount,
			Currency:          version.Currency,
			GoodsAmount:       version.GoodsAmount,
			ShippingAmount:    version.ShippingAmount,
			TaxAmount:         version.TaxAmount,
			ClientRequestID:   in.ClientRequestID,
			RequestHash:       version.PayloadHash,
			AcceptedAt:        now,
		})
		if insertErr != nil {
			return insertErr
		}
		payload, marshalErr := json.Marshal(OrderShellRequestedPayload{
			SchemaVersion: 1,
			AcceptanceID:  strconv.FormatInt(inserted.ID, 10),
			BuyerID:       strconv.FormatInt(int64(in.BuyerID), 10),
			RequestID:     strconv.FormatInt(in.RequestID, 10),
		})
		if marshalErr != nil {
			return marshalErr
		}
		// 冪等重送時 InsertBuyerQuoteAcceptance 回的是既有那一列，這裡會拿同一個
		// acceptance id 再插一次；uq_propo_acceptance_event 把它擋在 DB，query 自帶
		// ON CONFLICT DO NOTHING ⇒ 不會產生第二筆建單命令。
		if outboxErr := uc.store.InsertOrderProvisioningOutbox(txCtx, inserted.ID, payload, now); outboxErr != nil {
			return outboxErr
		}
		acceptance = inserted
		return nil
	})
	if err != nil {
		// ⛔ 這一段必須在交易外（wire 契約 §5.6）：23505 已讓原交易進入 aborted
		// （25P02），任何回查都會被自己觸發的錯誤打掉。
		// ⚠️ 2026-09-14 改包 WithTx 之後這條仍然成立，但理由換了一個：原本靠的是
		// 「這支 usecase 刻意不包交易」，現在靠的是 WithTx 在 fn 回錯時**已經 rollback
		// 並關掉交易**（pkg/database/tx_manager.go），而下面這行用的是外層 ctx——
		// 那個 ctx 裡沒有 tx，GetDBTX 會落回 pool。改動這一段前先確認這兩件事都還成立。
		if errors.Is(err, repository.ErrRequestAlreadyHasLiveAcceptance) {
			return AcceptSellerQuoteResult{}, uc.classifyLiveAcceptance(ctx, in.BuyerID, in.RequestID)
		}
		return AcceptSellerQuoteResult{}, err
	}
	// 撞到 (buyer_id, client_request_id) 冪等鍵時回的是既有那一列，它的 accepted_at
	// 是當初那次的時點，不是這次的 now。
	return AcceptSellerQuoteResult{Acceptance: acceptance, Replayed: !acceptance.AcceptedAt.Equal(now)}, nil
}

// classifyLiveAcceptance 回查既有那筆 live acceptance 是 ACTIVE 還是
// PAYMENT_FINALIZED，決定回哪一個碼。硬封鎖只發生在已 payment-finalized 的 award 上
// ——把封鎖點畫在 acceptance 會讓一次付款失敗永久鎖死這張 RFQ（wire 契約 §5.4-4）。
func (uc *BuyerAcceptanceUseCase) classifyLiveAcceptance(
	ctx context.Context, buyerID identity.BuyerID, requestID int64,
) error {
	existing, err := uc.store.FindLiveBuyerQuoteAcceptanceForRequest(ctx, buyerID, requestID)
	if err != nil {
		if errors.Is(err, repository.ErrBuyerQuoteAcceptanceNotFound) {
			// 回查時那筆已經被釋放：呼叫端重試就會成功，不是永久狀態。
			return ErrRfqCheckoutAlreadyActive
		}
		return err
	}
	if existing.State == repository.AcceptanceStatePaymentFinalized {
		return ErrRfqAlreadyAwarded
	}
	return ErrRfqCheckoutAlreadyActive
}
