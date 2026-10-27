package commerce

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/pricing"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

var (
	// ErrStalePublication 對應 STALE_PUBLICATION：publication id／version／
	// seller-facing payload hash 三者任一 CAS 不符。
	ErrStalePublication = errors.New("procurement: publication changed since it was read")

	// ErrSellerResponseAlreadySubmitted 對應 RFQ_RESPONSE_ALREADY_SUBMITTED：
	// 帶 expectedSellerResponseVersion="" 但這個賣家已經有正式回價。
	ErrSellerResponseAlreadySubmitted = errors.New("procurement: seller already has a submitted response on this publication")

	// ErrIdempotencyKeyRequired：本票四支寫入命令的冪等鍵走 Idempotency-Key header，
	// 缺 header 回 400（wire 契約 §5.1）。
	ErrIdempotencyKeyRequired = errors.New("procurement: Idempotency-Key is required")

	// ErrVerificationRequired：站外業者的送出、撤回、婉拒必須帶完整的驗證紀錄。
	ErrVerificationRequired = errors.New("procurement: vendor verification evidence is required")
)

// SellerResponseStore 是四支賣家命令需要的持久層能力。簽章逐字對齊
// repository.Repo，沿用本套件慣例（*repository.Repo 直接滿足這個介面）。
type SellerResponseStore interface {
	FindPublicationByID(ctx context.Context, id int64) (repository.Publication, error)
	FindPublicationInviteByID(ctx context.Context, id int64) (repository.PublicationInvite, error)
	FindPublicationInviteForStore(ctx context.Context, publicationID, storeID int64) (repository.PublicationInvite, error)
	ListPublicationsForStore(ctx context.Context, storeID int64, limit int32) ([]repository.PublicationForStore, error)

	FindConfirmedRfqSnapshotByIDAndBuyer(ctx context.Context, id int64, buyerID identity.BuyerID) (repository.ConfirmedRfqSnapshot, error)
	FindPricePreviewByIDAndBuyer(ctx context.Context, id int64, buyerID identity.BuyerID) (repository.PricePreview, error)

	EnsureSellerResponseIdentityForStore(ctx context.Context, publicationID int64, buyerID identity.BuyerID, requestID int64, storeID int64) (repository.SellerResponseIdentity, error)
	EnsureSellerResponseIdentityForProspect(ctx context.Context, publicationID int64, buyerID identity.BuyerID, requestID int64, prospectID int64) (repository.SellerResponseIdentity, error)
	FindSellerResponseByID(ctx context.Context, id int64) (repository.SellerResponseIdentity, error)
	UpsertSellerResponseDraft(ctx context.Context, sellerResponseID int64, payload json.RawMessage, expectedVersion int32) (repository.SellerResponseDraft, error)
	FindSellerResponseDraft(ctx context.Context, sellerResponseID int64) (repository.SellerResponseDraft, error)
	FindSellerResponseVersion(ctx context.Context, sellerResponseID int64, version int32) (repository.SellerResponseVersion, error)
	FindSellerResponseVersionByIdempotencyKey(ctx context.Context, sellerResponseID int64, clientRequestID string) (repository.SellerResponseVersion, error)
	GetPublicationInviteSellerView(ctx context.Context, inviteID int64) (repository.InviteSellerView, error)
	InsertSellerResponseVersion(ctx context.Context, in repository.InsertSellerResponseVersionInput) (repository.SellerResponseVersion, error)
	InsertSellerResponseCommand(ctx context.Context, in repository.InsertSellerResponseCommandInput) (repository.SellerResponseCommand, bool, error)
	FindSellerResponseCommandByIdempotencyKey(ctx context.Context, sellerResponseID int64, clientRequestID string) (repository.SellerResponseCommand, error)
	BumpSellerResponseLatestVersion(ctx context.Context, sellerResponseID int64, expectedVersion, latestVersion int32) (repository.SellerResponseIdentity, error)
	WithdrawSellerResponse(ctx context.Context, sellerResponseID int64, expectedVersion, expectedLatestVersion int32) (repository.SellerResponseIdentity, error)
	CloseSellerResponseWrite(ctx context.Context, sellerResponseID int64, expectedVersion int32, reason string) (repository.SellerResponseIdentity, error)
}

type SellerResponseUseCase struct {
	store SellerResponseStore
	tx    TxRunner
	now   func() time.Time
}

func NewSellerResponseUseCase(store SellerResponseStore, tx TxRunner, now func() time.Time) *SellerResponseUseCase {
	if now == nil {
		now = time.Now
	}
	return &SellerResponseUseCase{store: store, tx: tx, now: now}
}

// SellerPrincipal 是命令的發話者身分。StoreID 與 InviteID 恰有一個非零：PLATFORM
// 賣家走 store（gateway 由 metadata interceptor 帶入），站外業者走 vendor portal
// session 綁定的 invite id（gateway 已先做過 session 相等檢查）。
type SellerPrincipal struct {
	StoreID  int64
	InviteID int64
}

func (p SellerPrincipal) IsExternal() bool { return p.InviteID != 0 }

// evidenceFor 回這個 principal 的命令該存的驗證紀錄：站外業者必須帶完整的紀錄，
// 否則 fail-closed；平台賣家沒有這份紀錄，傳進來的一律不存。
func evidenceFor(p SellerPrincipal, v *repository.SellerResponseVerification) (*repository.SellerResponseVerification, error) {
	if !p.IsExternal() {
		return nil, nil
	}
	if v == nil || !v.Complete() {
		return nil, ErrVerificationRequired
	}
	out := *v
	out.VerifiedAt = out.VerifiedAt.UTC()
	return &out, nil
}

type SubmitSellerResponseInput struct {
	Principal                       SellerPrincipal
	PublicationID                   int64
	ClientRequestID                 string
	ExpectedPublicationVersion      string
	ExpectedSellerFacingPayloadHash string
	// ExpectedSellerResponseVersion "" ＝「我還沒有正式回價」。
	ExpectedSellerResponseVersion string
	Quote                         domain.SellerQuotePayload
	Verification                  *repository.SellerResponseVerification
}

type SellerResponseVersionResult struct {
	Identity repository.SellerResponseIdentity
	Version  repository.SellerResponseVersion
	Status   string
	Replayed bool
}

type UpsertSellerResponseDraftInput struct {
	Principal            SellerPrincipal
	PublicationID        int64
	ExpectedDraftVersion string
	Draft                domain.SellerQuotePayload
}

type WithdrawSellerResponseInput struct {
	Principal                     SellerPrincipal
	PublicationID                 int64
	SellerResponseID              int64
	ClientRequestID               string
	ExpectedSellerResponseVersion string
	ReasonCode                    string
	Verification                  *repository.SellerResponseVerification
}

type DeclineRfqPublicationInput struct {
	Principal       SellerPrincipal
	PublicationID   int64
	ClientRequestID string
	ReasonCode      string
	Verification    *repository.SellerResponseVerification
}

// ListPublications 回這個平台賣家看得到的 publication。WISH_POOL 拉模式沒有 invite
// 列（#2539 Slice 2c 未落地），所以目前只列得出 DIRECT lane 的。
type SellerPublicationListEntry struct {
	Row                  repository.PublicationForStore
	LatestResponseStatus string
	HasDraft             bool
}

func (uc *SellerResponseUseCase) ListPublications(ctx context.Context, storeID int64, limit int32) ([]SellerPublicationListEntry, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := uc.store.ListPublicationsForStore(ctx, storeID, limit)
	if err != nil {
		return nil, err
	}
	now := uc.now().UTC()
	out := make([]SellerPublicationListEntry, 0, len(rows))
	for _, row := range rows {
		entry := SellerPublicationListEntry{Row: row, HasDraft: row.HasDraft}
		if row.LatestVersion != nil && *row.LatestVersion > 0 && row.LatestVersionValidUntil != nil {
			entry.LatestResponseStatus = domain.SellerResponseVersionStatus(
				*row.LatestVersion, *row.LatestVersion, row.WithdrawnVersion,
				*row.LatestVersionValidUntil, now,
			)
		}
		out = append(out, entry)
	}
	return out, nil
}

// GetPublication 解析一張 publication，並驗證這個賣家看得到它。跨賣家猜 id 一律
// ErrPublicationNotFound（→404）——403 會洩漏該 id 存在（doc 64 §9.3）。
func (uc *SellerResponseUseCase) GetPublication(
	ctx context.Context, p SellerPrincipal, publicationID int64,
) (repository.Publication, repository.PublicationInvite, error) {
	invite, err := uc.resolveVisibility(ctx, p, publicationID)
	if err != nil {
		return repository.Publication{}, repository.PublicationInvite{}, err
	}
	pub, err := uc.store.FindPublicationByID(ctx, invite.PublicationID)
	if err != nil {
		return repository.Publication{}, repository.PublicationInvite{}, err
	}
	return pub, invite, nil
}

// resolveVisibility 把 principal 換成「這個賣家在這張 publication 上的收件人身分」。
// EXTERNAL 走 invite id：gateway 已驗過 session 綁定，這裡只再確認它確實掛在這張
// publication 上——不符一律當成看不到。
func (uc *SellerResponseUseCase) resolveVisibility(
	ctx context.Context, p SellerPrincipal, publicationID int64,
) (repository.PublicationInvite, error) {
	if p.IsExternal() {
		invite, err := uc.store.FindPublicationInviteByID(ctx, p.InviteID)
		if err != nil {
			return repository.PublicationInvite{}, err
		}
		if publicationID != 0 && invite.PublicationID != publicationID {
			return repository.PublicationInvite{}, repository.ErrInviteNotFound
		}
		return invite, nil
	}
	return uc.store.FindPublicationInviteForStore(ctx, publicationID, p.StoreID)
}

// ensureIdentity 取得（必要時建立）這個賣家在這張 publication 上的身分列。
func (uc *SellerResponseUseCase) ensureIdentity(
	ctx context.Context, p SellerPrincipal, pub repository.Publication, invite repository.PublicationInvite,
) (repository.SellerResponseIdentity, error) {
	if p.IsExternal() {
		return uc.store.EnsureSellerResponseIdentityForProspect(ctx, pub.ID, pub.BuyerID, pub.RequestID, invite.ProspectID)
	}
	return uc.store.EnsureSellerResponseIdentityForStore(ctx, pub.ID, pub.BuyerID, pub.RequestID, p.StoreID)
}

// UpsertDraft 存一版草稿。草稿不計正式一次、不驗金額恆等式，也不得被買家讀到。
// CAS 對象是 expectedDraftVersion，所以不需要冪等鍵。
func (uc *SellerResponseUseCase) UpsertDraft(
	ctx context.Context, in UpsertSellerResponseDraftInput,
) (repository.SellerResponseDraft, error) {
	pub, invite, err := uc.GetPublication(ctx, in.Principal, in.PublicationID)
	if err != nil {
		return repository.SellerResponseDraft{}, err
	}
	expected, err := parseOptionalVersion(in.ExpectedDraftVersion)
	if err != nil {
		return repository.SellerResponseDraft{}, err
	}
	payload, err := json.Marshal(in.Draft)
	if err != nil {
		return repository.SellerResponseDraft{}, err
	}

	var draft repository.SellerResponseDraft
	err = uc.tx.WithTx(ctx, func(txCtx context.Context) error {
		identity, txErr := uc.ensureIdentity(txCtx, in.Principal, pub, invite)
		if txErr != nil {
			return txErr
		}
		if identity.WriteClosed() {
			return repository.ErrSellerResponseWriteClosed
		}
		draft, txErr = uc.store.UpsertSellerResponseDraft(txCtx, identity.ID, payload, expected)
		return txErr
	})
	if err != nil {
		return repository.SellerResponseDraft{}, err
	}
	return draft, nil
}

// SellerPublicationDetail 是賣家詳情：發布本體，加上這個賣家自己的身分列、草稿與
// 目前正式版本。HasDraft／HasLatest 為 false 時對應欄位是零值，不是「查詢失敗」。
type SellerPublicationDetail struct {
	Publication  repository.Publication
	Invite       repository.PublicationInvite
	Identity     repository.SellerResponseIdentity
	Draft        repository.SellerResponseDraft
	HasDraft     bool
	Latest       repository.SellerResponseVersion
	HasLatest    bool
	LatestStatus string
	// SellerView 是賣家在報什麼價的九欄白名單。取不到時是零值——寧可少顯示一張
	// 需求卡，也不讓整支詳情失敗。
	SellerView repository.InviteSellerView
}

// GetPublicationDetail 一次交出賣家填 CAS 欄位需要的全部東西：expectedDraftVersion
// 來自 Draft.Version、expectedSellerResponseVersion 來自 Identity.LatestVersion。
// 少了這一支，賣家端只能猜。
func (uc *SellerResponseUseCase) GetPublicationDetail(
	ctx context.Context, p SellerPrincipal, publicationID int64,
) (SellerPublicationDetail, error) {
	pub, invite, err := uc.GetPublication(ctx, p, publicationID)
	if err != nil {
		return SellerPublicationDetail{}, err
	}
	id, err := uc.ensureIdentity(ctx, p, pub, invite)
	if err != nil {
		return SellerPublicationDetail{}, err
	}
	out := SellerPublicationDetail{Publication: pub, Invite: invite, Identity: id}

	// 需求內容走 vendor portal 已上線的同一支讀取：兩個入口看到的必須是同一份
	// 白名單內容。取不到不讓整支失敗——回價表單少一張需求卡仍可用，整支 500 不行。
	if view, viewErr := uc.store.GetPublicationInviteSellerView(ctx, invite.ID); viewErr == nil {
		out.SellerView = view
	} else if !errors.Is(viewErr, repository.ErrInviteNotFound) {
		return SellerPublicationDetail{}, viewErr
	}

	draft, err := uc.store.FindSellerResponseDraft(ctx, id.ID)
	switch {
	case err == nil:
		out.Draft, out.HasDraft = draft, true
	case errors.Is(err, repository.ErrSellerResponseNotFound):
		// 還沒存過草稿是常態，不是錯誤。
	default:
		return SellerPublicationDetail{}, err
	}

	if id.HasSubmission() {
		latest, verErr := uc.store.FindSellerResponseVersion(ctx, id.ID, id.LatestVersion)
		if verErr != nil {
			return SellerPublicationDetail{}, verErr
		}
		out.Latest, out.HasLatest = latest, true
		out.LatestStatus = domain.SellerResponseVersionStatus(
			latest.Version, id.LatestVersion, id.WithdrawnVersion, latest.ValidUntil, uc.now().UTC(),
		)
	}
	return out, nil
}

// GetDraft 讀回自己的草稿。
func (uc *SellerResponseUseCase) GetDraft(
	ctx context.Context, p SellerPrincipal, publicationID int64,
) (repository.SellerResponseDraft, error) {
	pub, invite, err := uc.GetPublication(ctx, p, publicationID)
	if err != nil {
		return repository.SellerResponseDraft{}, err
	}
	identity, err := uc.ensureIdentity(ctx, p, pub, invite)
	if err != nil {
		return repository.SellerResponseDraft{}, err
	}
	return uc.store.FindSellerResponseDraft(ctx, identity.ID)
}

// Submit 是正式提交與改版的唯一入口（wire 契約 §5.2）：語意是「在這個 publication
// 上，為我這個賣家身分追加下一個 immutable version」，append-only 因此是它唯一能做
// 的事。
func (uc *SellerResponseUseCase) Submit(
	ctx context.Context, in SubmitSellerResponseInput,
) (SellerResponseVersionResult, error) {
	evidence, err := evidenceFor(in.Principal, in.Verification)
	if err != nil {
		return SellerResponseVersionResult{}, err
	}
	if in.ClientRequestID == "" {
		return SellerResponseVersionResult{}, ErrIdempotencyKeyRequired
	}
	pub, invite, err := uc.GetPublication(ctx, in.Principal, in.PublicationID)
	if err != nil {
		return SellerResponseVersionResult{}, err
	}
	if err := checkPublicationCAS(pub, in.ExpectedPublicationVersion, in.ExpectedSellerFacingPayloadHash); err != nil {
		return SellerResponseVersionResult{}, err
	}

	now := uc.now().UTC()
	quote, err := in.Quote.Normalize(now)
	if err != nil {
		return SellerResponseVersionResult{}, err
	}
	expectedVersion, err := parseOptionalVersion(in.ExpectedSellerResponseVersion)
	if err != nil {
		return SellerResponseVersionResult{}, err
	}
	outsideRange, err := uc.outsideBuyerRequestedRange(ctx, pub, quote)
	if err != nil {
		return SellerResponseVersionResult{}, err
	}

	var result SellerResponseVersionResult
	err = uc.tx.WithTx(ctx, func(txCtx context.Context) error {
		id, txErr := uc.ensureIdentity(txCtx, in.Principal, pub, invite)
		if txErr != nil {
			return txErr
		}

		// ⛔ 回放查找必須排在 CAS 與 write-closed 兩道守門之前：第一次成功已把
		// latest_version 推上去，先跑 CAS 會把重試判成 RFQ_RESPONSE_ALREADY_SUBMITTED。
		replay, txErr := uc.store.FindSellerResponseVersionByIdempotencyKey(txCtx, id.ID, in.ClientRequestID)
		switch {
		case txErr == nil:
			if replay.PayloadHash != quote.PayloadHash {
				return repository.ErrSellerResponseIdempotencyConflict
			}
			result = SellerResponseVersionResult{Identity: id, Version: replay, Replayed: true}
			return nil
		case errors.Is(txErr, repository.ErrSellerResponseNotFound):
		default:
			return txErr
		}

		if id.WriteClosed() {
			return repository.ErrSellerResponseWriteClosed
		}
		if expectedVersion == 0 && id.HasSubmission() {
			return ErrSellerResponseAlreadySubmitted
		}
		if expectedVersion != 0 && id.LatestVersion != expectedVersion {
			return repository.ErrSellerResponseStale
		}

		next := id.LatestVersion + 1
		version, txErr := uc.store.InsertSellerResponseVersion(txCtx, repository.InsertSellerResponseVersionInput{
			SellerResponseID:              id.ID,
			Version:                       next,
			ResponseType:                  quote.Payload.ResponseType,
			OfferedQualityGrade:           quote.Payload.OfferedQuality.Grade,
			OfferedQualityInspectionNotes: quote.Payload.OfferedQuality.InspectionNotes,
			UnitBasisQuantity:             quote.Payload.UnitBasis.Quantity,
			UnitBasisUnit:                 quote.Payload.UnitBasis.Unit,
			OfferedQuantity:               quote.Payload.OfferedQuantity,
			OfferedQuantityUnit:           quote.Payload.OfferedQuantityUnit,
			ConversionBasis:               quote.Payload.ConversionBasis,
			Currency:                      quote.Payload.Currency,
			GoodsAmount:                   quote.GoodsAmount,
			ShippingAmount:                quote.ShippingAmount,
			TaxAmount:                     quote.TaxAmount,
			TaxInclusionBasis:             quote.Payload.TaxInclusionBasis,
			TotalAmount:                   quote.TotalAmount,
			LeadTimeDays:                  quote.LeadTimeDays,
			PromisedDeliveryAt:            quote.PromisedDeliveryAt,
			ValidUntil:                    quote.ValidUntil,
			TermsNote:                     quote.Payload.TermsNote,
			DeviationDisclosure:           quote.Payload.DeviationDisclosure,
			OutsideBuyerRequestedRange:    outsideRange,
			PayloadHash:                   quote.PayloadHash,
			PublicationVersion:            int32(pub.Version),
			SellerFacingPayloadHash:       pub.SellerFacingPayloadHash,
			ClientRequestID:               in.ClientRequestID,
			SubmittedAt:                   now,
			Verification:                  evidence,
		})
		if txErr != nil {
			return txErr
		}

		// version.Version != next ⇒ 撞到 (seller_response_id, client_request_id)
		// 冪等鍵、回放的是既有那一版。回放不得再 bump latest_version：那一版當初
		// 已經 bump 過，再做一次會把身分列推到一個沒有對應版本列的號碼。
		if version.Version != next {
			result = SellerResponseVersionResult{Identity: id, Version: version, Replayed: true}
			return nil
		}
		bumped, txErr := uc.store.BumpSellerResponseLatestVersion(txCtx, id.ID, id.Version, next)
		if txErr != nil {
			return txErr
		}
		result = SellerResponseVersionResult{Identity: bumped, Version: version}
		return nil
	})
	if err != nil {
		return SellerResponseVersionResult{}, err
	}
	result.Status = domain.SellerResponseVersionStatus(
		result.Version.Version, result.Identity.LatestVersion, result.Identity.WithdrawnVersion,
		result.Version.ValidUntil, now,
	)
	return result, nil
}

// Withdraw 撤回目前最新的正式回價。舊 payload 不改寫，版本列本身永不 UPDATE
// （wire 契約 §5.4-2）；撤回的事實先追加成指令紀錄，身分列的 withdrawn_version
// 在同一個 transaction 由那筆紀錄推出。
func (uc *SellerResponseUseCase) Withdraw(
	ctx context.Context, in WithdrawSellerResponseInput,
) (repository.SellerResponseIdentity, error) {
	evidence, err := evidenceFor(in.Principal, in.Verification)
	if err != nil {
		return repository.SellerResponseIdentity{}, err
	}
	if in.ClientRequestID == "" {
		return repository.SellerResponseIdentity{}, ErrIdempotencyKeyRequired
	}
	pub, invite, err := uc.GetPublication(ctx, in.Principal, in.PublicationID)
	if err != nil {
		return repository.SellerResponseIdentity{}, err
	}
	expectedVersion, err := parseOptionalVersion(in.ExpectedSellerResponseVersion)
	if err != nil {
		return repository.SellerResponseIdentity{}, err
	}

	var out repository.SellerResponseIdentity
	err = uc.tx.WithTx(ctx, func(txCtx context.Context) error {
		id, txErr := uc.ensureIdentity(txCtx, in.Principal, pub, invite)
		if txErr != nil {
			return txErr
		}
		// 路徑上的 sellerResponseId 必須是這個賣家自己的身分列；不符當成看不到，
		// 不回述請求的 id。
		if in.SellerResponseID != 0 && in.SellerResponseID != id.ID {
			return repository.ErrSellerResponseNotFound
		}

		// 回放查找排在 write-closed 與版本守門之前，理由同 Submit。
		replay, txErr := uc.store.FindSellerResponseCommandByIdempotencyKey(txCtx, id.ID, in.ClientRequestID)
		switch {
		case txErr == nil:
			if !sameWithdrawCommand(replay, expectedVersion, in.ReasonCode) {
				return repository.ErrSellerResponseIdempotencyConflict
			}
			out = withdrawOutcome(id, replay)
			return nil
		case errors.Is(txErr, repository.ErrSellerResponseNotFound):
		default:
			return txErr
		}

		if id.WriteClosed() {
			return repository.ErrSellerResponseWriteClosed
		}
		if id.WithdrawnVersion != nil && *id.WithdrawnVersion == expectedVersion {
			out = id // 冪等：同一版重複撤回是同一個結果
			return nil
		}
		if id.LatestVersion != expectedVersion || !id.HasSubmission() {
			return repository.ErrSellerResponseStale
		}
		cmd, created, txErr := uc.store.InsertSellerResponseCommand(txCtx, repository.InsertSellerResponseCommandInput{
			SellerResponseID: id.ID,
			Kind:             repository.SellerResponseCommandWithdraw,
			WithdrawnVersion: &expectedVersion,
			ClientRequestID:  in.ClientRequestID,
			ReasonCode:       in.ReasonCode,
			Verification:     evidence,
		})
		if txErr != nil {
			return txErr
		}
		if !created {
			if !sameWithdrawCommand(cmd, expectedVersion, in.ReasonCode) {
				return repository.ErrSellerResponseIdempotencyConflict
			}
			out = withdrawOutcome(id, cmd)
			return nil
		}
		out, txErr = uc.store.WithdrawSellerResponse(txCtx, id.ID, id.Version, *cmd.WithdrawnVersion)
		return txErr
	})
	if err != nil {
		return repository.SellerResponseIdentity{}, err
	}
	return out, nil
}

// Decline 明確拒絕。終止該賣家在此 publication 的寫入，冪等，理由可選。
func (uc *SellerResponseUseCase) Decline(
	ctx context.Context, in DeclineRfqPublicationInput,
) (repository.SellerResponseIdentity, error) {
	evidence, err := evidenceFor(in.Principal, in.Verification)
	if err != nil {
		return repository.SellerResponseIdentity{}, err
	}
	if in.ClientRequestID == "" {
		return repository.SellerResponseIdentity{}, ErrIdempotencyKeyRequired
	}
	pub, invite, err := uc.GetPublication(ctx, in.Principal, in.PublicationID)
	if err != nil {
		return repository.SellerResponseIdentity{}, err
	}

	var out repository.SellerResponseIdentity
	err = uc.tx.WithTx(ctx, func(txCtx context.Context) error {
		id, txErr := uc.ensureIdentity(txCtx, in.Principal, pub, invite)
		if txErr != nil {
			return txErr
		}
		replay, txErr := uc.store.FindSellerResponseCommandByIdempotencyKey(txCtx, id.ID, in.ClientRequestID)
		switch {
		case txErr == nil:
			if !sameDeclineCommand(replay, in.ReasonCode) {
				return repository.ErrSellerResponseIdempotencyConflict
			}
			out = id
			return nil
		case errors.Is(txErr, repository.ErrSellerResponseNotFound):
		default:
			return txErr
		}

		// 已關閉即回既有列：decline 是冪等命令，重送不是錯誤。已被別的理由關閉
		// （PUBLICATION_CLOSED／OTHER_WON）同樣回既有列——寫入權已終止是同一個
		// 對外事實，不因關閉理由不同而改變。
		if id.WriteClosed() {
			out = id
			return nil
		}
		cmd, created, txErr := uc.store.InsertSellerResponseCommand(txCtx, repository.InsertSellerResponseCommandInput{
			SellerResponseID: id.ID,
			Kind:             repository.SellerResponseCommandDecline,
			ClientRequestID:  in.ClientRequestID,
			ReasonCode:       in.ReasonCode,
			Verification:     evidence,
		})
		if txErr != nil {
			return txErr
		}
		if !created {
			if !sameDeclineCommand(cmd, in.ReasonCode) {
				return repository.ErrSellerResponseIdempotencyConflict
			}
			out = id
			return nil
		}
		out, txErr = uc.store.CloseSellerResponseWrite(txCtx, id.ID, id.Version, repository.SellerResponseClosedDeclined)
		return txErr
	})
	if err != nil {
		return repository.SellerResponseIdentity{}, err
	}
	return out, nil
}

// withdrawOutcome 由指令紀錄還原撤回當下的結果：撤回要求 latest_version 等於被撤回的
// 版本，且當時寫入未關閉；之後的改版不得讓回放看起來像沒撤回過。
func withdrawOutcome(live repository.SellerResponseIdentity, c repository.SellerResponseCommand) repository.SellerResponseIdentity {
	out := live
	out.LatestVersion = *c.WithdrawnVersion
	out.WithdrawnVersion = c.WithdrawnVersion
	at := c.CreatedAt
	out.WithdrawnAt = &at
	out.WriteClosedAt = nil
	out.WriteClosedReason = nil
	return out
}

func sameWithdrawCommand(c repository.SellerResponseCommand, expectedVersion int32, reasonCode string) bool {
	return c.Kind == repository.SellerResponseCommandWithdraw &&
		c.WithdrawnVersion != nil && *c.WithdrawnVersion == expectedVersion &&
		c.ReasonCode == reasonCode
}

func sameDeclineCommand(c repository.SellerResponseCommand, reasonCode string) bool {
	return c.Kind == repository.SellerResponseCommandDecline && c.ReasonCode == reasonCode
}

func checkPublicationCAS(pub repository.Publication, expectedVersion, expectedPayloadHash string) error {
	if expectedVersion != "" {
		v, err := strconv.ParseInt(expectedVersion, 10, 32)
		if err != nil || int(v) != pub.Version {
			return ErrStalePublication
		}
	}
	if expectedPayloadHash != "" && expectedPayloadHash != pub.SellerFacingPayloadHash {
		return ErrStalePublication
	}
	return nil
}

// parseOptionalVersion 把 wire 上的十進位字串版本轉成 int32，"" 回 0（＝「還沒有」）。
func parseOptionalVersion(s string) (int32, error) {
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(s, 10, 32)
	if err != nil || v <= 0 {
		return 0, repository.ErrSellerResponseStale
	}
	return int32(v), nil
}

// outsideBuyerRequestedRange 是**只標示**的旗標（wire 契約 §6.2：不得據此篡改賣家
// 金額，也不得阻擋送出）。
//
// ⚠️ 只在「賣家的 unitBasis 與買家區間的計價基準完全同基準」時才判定，其餘一律 false。
// 兩邊基準不同時要判定就得由 server 發明一次單位換算，那會產出一個看起來權威、實際
// 是伺服器猜的價格比較——`conversionBasis` 這個欄位存在就是因為這個換算屬於賣家的
// 聲明，不是伺服器的推導。因此 false 的語意是「未判定為超出」，不是「已確認在範圍內」。
//
// buyerRequestedRange 不在 seller-facing payload 裡（domain.SellerFacingPayload 是
// 白名單型別），所以這裡從 publication → confirmed snapshot → price preview 讀，
// 全程 server-side，不回給賣家。
func (uc *SellerResponseUseCase) outsideBuyerRequestedRange(
	ctx context.Context, pub repository.Publication, quote domain.NormalizedSellerQuote,
) (bool, error) {
	snapshot, err := uc.store.FindConfirmedRfqSnapshotByIDAndBuyer(ctx, pub.ConfirmedRfqSnapshotID, pub.BuyerID)
	if err != nil {
		if errors.Is(err, repository.ErrConfirmedRfqSnapshotNotFound) {
			return false, nil
		}
		return false, err
	}
	preview, err := uc.store.FindPricePreviewByIDAndBuyer(ctx, snapshot.PricePreviewID, pub.BuyerID)
	if err != nil {
		if errors.Is(err, repository.ErrPricePreviewNotFound) {
			return false, nil
		}
		return false, err
	}
	if len(preview.BuyerRequestedRangeJSON) == 0 {
		return false, nil
	}
	var rng pricing.BuyerRequestedRange
	if err := json.Unmarshal(preview.BuyerRequestedRangeJSON, &rng); err != nil {
		return false, nil
	}
	if rng.BasisUnit == "" || rng.BasisUnit != quote.Payload.UnitBasis.Unit ||
		rng.BasisQuantity != quote.Payload.UnitBasis.Quantity {
		return false, nil
	}

	lower, lowerErr := strconv.ParseInt(rng.LowerBoundAmount, 10, 64)
	upper, upperErr := strconv.ParseInt(rng.UpperBoundAmount, 10, 64)
	basisQty, basisErr := strconv.ParseInt(rng.BasisQuantity, 10, 64)
	offeredQty, offeredErr := strconv.ParseInt(quote.Payload.OfferedQuantity, 10, 64)
	if lowerErr != nil || upperErr != nil || basisErr != nil || offeredErr != nil ||
		basisQty <= 0 || offeredQty <= 0 || offeredQty%basisQty != 0 {
		return false, nil
	}

	// 區間是「每 basisQuantity 個 basisUnit」的價格，所以放大到這次報的數量再比
	// goods（不含運費與稅——區間談的是貨款）。整除已在上面確認，這裡不會有捨入。
	multiple := offeredQty / basisQty
	return quote.GoodsAmount < lower*multiple || quote.GoodsAmount > upper*multiple, nil
}
