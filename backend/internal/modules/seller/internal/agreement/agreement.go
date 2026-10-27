// Package agreement 記錄賣家入駐時同意的文件（seller.agreement_acceptances，只有 INSERT）。
package agreement

import (
	"context"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
)

// Source 是同意當下的來源：IP 與 User-Agent，取不到是空字串（存成 NULL）。
type Source struct {
	ClientIP, UserAgent string
}

// Subject 是同意紀錄綁定的來源，依種類恰好一個（資料表的 CHECK 把關）：KYC 那次送出的 SUBMIT 審核紀錄（SubmissionReviewID），
// 食品業者登錄那次送出建立的登錄（FoodRegistrationID）；開店的 SELLER_TERMS 兩者都是 nil。
type Subject struct {
	SubmissionReviewID, FoodRegistrationID *uuid.UUID
}

// Record 在呼叫方的 transaction 內寫入同意紀錄，每次同意事件一列。呼叫方先以 model.CheckAgreements 核對過。
func Record(ctx context.Context, q *store.Queries, storeID, accountID int64, accepted []model.AgreementAcceptance, at time.Time, subject Subject, src Source) error {
	ip, ua := model.ClientIPAddr(src.ClientIP), model.UserAgentText(src.UserAgent)
	for _, a := range accepted {
		if err := q.InsertAgreementAcceptance(ctx, store.InsertAgreementAcceptanceParams{
			StoreID: storeID, Kind: string(a.Kind), Version: a.Version, AccountID: accountID,
			AcceptedAt: at, SubmissionReviewID: subject.SubmissionReviewID, FoodRegistrationID: subject.FoodRegistrationID, ClientIp: ip, UserAgent: ua,
		}); err != nil {
			return err
		}
	}
	return nil
}
