package foodreg

import (
	"context"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
)

// reviewRow 是收件匣、歷史清單與單筆檢視共用的列：登錄與店家。sortAt 只有「現在可以裁定」隊列有（期限，游標的排序時間）。
type reviewRow struct {
	reg         store.SellerFoodRegistration
	storeName   *string
	storeStatus string
	sortAt      time.Time
}

// related 是一批列的店家各自的關聯資料：目前核准的登錄、KYC 申請的狀態，以及已核准申請的主體。
type related struct {
	approved map[int64]*model.FoodRegistration
	kyc      map[int64]model.KYCStatus
	subjects map[int64]model.FoodRegistrationSubject
	// deadlines 是登錄各自的裁定期限（seller.review_food_registration）；現在不能裁定的沒有。
	deadlines map[uuid.UUID]*model.Deadline
}

// loadRelated 一次讀出 rows 的店家的核准登錄、KYC 狀態與已核准的 KYC 主體。主體只來自 APPROVED 的申請（ApprovedKYCSubjects）：
// 賣家還沒送出或還在審的草稿沒有被驗證過，後台看不到。
func loadRelated(ctx context.Context, q *store.Queries, rows []reviewRow, now time.Time) (related, error) {
	ids := make([]int64, 0, len(rows))
	regIDs := make([]uuid.UUID, 0, len(rows))
	seen := make(map[int64]bool, len(rows))
	for _, r := range rows {
		regIDs = append(regIDs, r.reg.ID)
		if !seen[r.reg.StoreID] {
			seen[r.reg.StoreID] = true
			ids = append(ids, r.reg.StoreID)
		}
	}
	out := related{
		approved:  make(map[int64]*model.FoodRegistration, len(ids)),
		kyc:       make(map[int64]model.KYCStatus, len(ids)),
		subjects:  make(map[int64]model.FoodRegistrationSubject, len(ids)),
		deadlines: make(map[uuid.UUID]*model.Deadline, len(regIDs)),
	}
	deadlines, err := q.ReviewFoodRegistrationByIDs(ctx, store.ReviewFoodRegistrationByIDsParams{At: now, Ids: regIDs})
	if err != nil {
		return related{}, err
	}
	for _, d := range deadlines {
		out.deadlines[d.FoodRegistrationID] = &model.Deadline{Start: d.StartAt, Due: d.DueAt}
	}
	regs, err := q.StoreFoodRegistrations(ctx, ids)
	if err != nil {
		return related{}, err
	}
	for _, row := range regs {
		if row.State == string(model.FoodRegApproved) {
			out.approved[row.StoreID] = stint.FoodRegistrationOf(row)
		}
	}
	statuses, err := q.CurrentKYCStatuses(ctx, ids)
	if err != nil {
		return related{}, err
	}
	for _, row := range statuses {
		out.kyc[row.StoreID] = model.KYCStatus(row.Status)
	}
	subjects, err := q.ApprovedKYCSubjects(ctx, ids)
	if err != nil {
		return related{}, err
	}
	for _, row := range subjects {
		out.subjects[row.StoreID] = model.FoodRegistrationSubject{
			SellerType: model.KYCSellerType(row.SellerType), RealName: model.Deref(row.RealName),
			CompanyName: model.Deref(row.CompanyName), TaxID: model.Deref(row.TaxID),
		}
	}
	return out, nil
}

func (r reviewRow) toReview(rel related) model.FoodRegistrationReview {
	out := model.FoodRegistrationReview{
		ID: r.reg.ID, StoreID: r.reg.StoreID, StoreStatus: model.Status(r.storeStatus), RegistrationNumber: model.Deref(r.reg.RegistrationNumber),
		State: model.FoodRegState(r.reg.State), SubmittedAt: r.reg.SubmittedAt, DecidedByAdminID: r.reg.DecidedByAdminID,
		DecidedAt: r.reg.DecidedAt, EndedAt: r.reg.EndedAt, Approved: rel.approved[r.reg.StoreID], DecideDeadline: rel.deadlines[r.reg.ID],
	}
	if r.storeName != nil {
		out.StoreName = *r.storeName
	}
	if r.reg.RejectReason != nil {
		out.RejectReason = *r.reg.RejectReason
	}
	if r.reg.InternalNote != nil {
		out.InternalNote = *r.reg.InternalNote
	}
	if r.reg.EndReason != nil {
		out.EndReason = *r.reg.EndReason
	}
	out.EndedByAdminID = r.reg.EndedByAdminID
	if r.reg.EndCause != nil {
		out.EndCause = model.FoodRegEndCause(*r.reg.EndCause)
	}
	if status, ok := rel.kyc[r.reg.StoreID]; ok {
		out.KYCStatus = &status
	}
	if subject, ok := rel.subjects[r.reg.StoreID]; ok {
		out.Subject = &subject
	}
	return out
}
