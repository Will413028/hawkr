package kyc

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
)

// SeedApproved 寫入一筆已核准的申請給本機開發種子（seller 的 SeedDevStores）：明文的輸入經 SaveDraft 同一支 applyDraft 驗證、加密並計算
// blind index（加密的 AAD 與 hash 的輸入只有這一份實作，金鑰是執行中的金鑰環），再以草稿插入、依序轉成 SUBMITTED 與 APPROVED（同一個 TransitionKYCApplication，submission_seq 因此與正式流程一致）。它不經送出與審核的
// 服務流程（只寫送出的 SUBMIT 審核紀錄，給入駐同意綁定；不寫核准的審核紀錄、不發事件、不排 job），與 SQL 種子一樣只給本機開發的資料用；呼叫方在自己的 transaction 內呼叫。
func (s *Service) SeedApproved(ctx context.Context, q *store.Queries, storeID int64, in model.KYCDraftInput, now time.Time) (store.SellerKycApplication, uuid.UUID, error) {
	now = now.UTC()
	row, _, err := s.p.applyDraft(store.SellerKycApplication{ApplicationID: uuid.NewV7(), StoreID: storeID}, in, true, now)
	if err != nil {
		return store.SellerKycApplication{}, uuid.Nil(), err
	}
	draft, err := q.InsertKYCApplication(ctx, insertParams(row, now))
	if err != nil {
		return store.SellerKycApplication{}, uuid.Nil(), err
	}
	if err := s.p.syncHashes(ctx, q, nil, draft, now); err != nil {
		return store.SellerKycApplication{}, uuid.Nil(), err
	}
	// 照正式流程的狀態轉移：草稿 → SUBMITTED（submission_seq 加一）→ APPROVED，用同一個 TransitionKYCApplication。
	submitted, err := q.TransitionKYCApplication(ctx, store.TransitionKYCApplicationParams{
		ApplicationID: draft.ApplicationID, ExpectedVersion: draft.Version, FromStatus: string(model.KYCStatusDraft),
		ToStatus: string(model.KYCStatusSubmitted), SubmittedAt: &now, Now: now,
	})
	if err != nil {
		return store.SellerKycApplication{}, uuid.Nil(), fmt.Errorf("seller：種子申請轉成送出：%w", err)
	}
	submit, err := q.InsertKYCReview(ctx, reviewParams(submitted.ApplicationID, model.ReviewSubmit, nil, "", "", nil, now))
	if err != nil {
		return store.SellerKycApplication{}, uuid.Nil(), fmt.Errorf("seller：種子申請的送出紀錄：%w", err)
	}
	approved, err := q.TransitionKYCApplication(ctx, store.TransitionKYCApplicationParams{
		ApplicationID: submitted.ApplicationID, ExpectedVersion: submitted.Version, FromStatus: string(model.KYCStatusSubmitted),
		ToStatus: string(model.KYCStatusApproved), DecidedAt: &now, Now: now,
	})
	if err != nil {
		return store.SellerKycApplication{}, uuid.Nil(), fmt.Errorf("seller：種子申請轉成核准：%w", err)
	}
	return approved, submit.ReviewID, nil
}
