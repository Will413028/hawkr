package kyc

import (
	"context"
	"errors"
	"slices"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/auditreason"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
)

// AdminList 是 admin 的審核佇列：只列送出過的申請（不含草稿，也不含沒送出就取消的），依狀態與關鍵字（店名、公司名稱、統編的
// 子字串，ILIKE）篩選，keyset，預設 20、上限 100。排序鍵是送出時間，方向由 model.KYCListInput.OldestFirst 決定：篩選審核中時
// 舊到新，其他新到舊。列名是店名（Stripe 計畫 D37）；清單的項目不帶補充文件。
func (s *Service) AdminList(ctx context.Context, in model.KYCListInput) (model.KYCPage, error) {
	size, err := pagination.PageSize(in.PageSize, 20, 100)
	if err != nil {
		return model.KYCPage{}, err
	}
	pattern := model.LikePattern(in.Query)
	var status *string
	if in.Status != nil {
		if !slices.Contains(model.KYCStatusValues(), *in.Status) || *in.Status == model.KYCStatusDraft {
			return model.KYCPage{}, apperr.InvalidInputOn([]string{"Status"}, "狀態不合法")
		}
		v := string(*in.Status)
		status = &v
	}
	var afterAt *time.Time
	var afterID *uuid.UUID
	if in.After != nil {
		afterAt, afterID = &in.After.SubmittedAt, &in.After.ApplicationID
	}
	q := store.New(s.d.DB.Conn(ctx))
	var rows []store.SellerKycApplication
	if in.OldestFirst() {
		rows, err = q.AdminKYCApplicationsOldestFirst(ctx, store.AdminKYCApplicationsOldestFirstParams{
			Status: status, QueryPattern: pattern, AfterSubmittedAt: afterAt, AfterID: afterID, PageSize: size + 1,
		})
	} else {
		rows, err = q.AdminKYCApplications(ctx, store.AdminKYCApplicationsParams{
			Status: status, QueryPattern: pattern, AfterSubmittedAt: afterAt, AfterID: afterID, PageSize: size + 1,
		})
	}
	if err != nil {
		return model.KYCPage{}, err
	}
	hasMore := len(rows) > int(size)
	if hasMore {
		rows = rows[:size]
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.StoreID)
	}
	owners, err := q.StoreAccounts(ctx, ids)
	if err != nil {
		return model.KYCPage{}, err
	}
	accounts := make(map[int64]store.StoreAccountsRow, len(owners))
	for _, o := range owners {
		accounts[o.StoreID] = o
	}
	page := model.KYCPage{Items: make([]model.KYCApplication, 0, len(rows))}
	for _, row := range rows {
		item := toApplication(row, accounts[row.StoreID].AccountID)
		item.StoreName = accounts[row.StoreID].StoreName
		page.Items = append(page.Items, item)
	}
	if hasMore {
		last := page.Items[len(page.Items)-1]
		// 佇列只有送出過的申請，SubmittedAt 一定有值。
		page.Next = &model.KYCCursor{SubmittedAt: *last.SubmittedAt, ApplicationID: last.ID}
	}
	return page, nil
}

// AdminGet 讀一筆申請的後台檢視：欄位、補充文件（asset 與位置；檢視網址由 handler 組）、審核紀錄。補充文件的讀取紀錄另外分頁（ListDocumentAccesses）。
// 草稿是賣家自己的，後台讀不到（NotFound）。
func (s *Service) AdminGet(ctx context.Context, applicationID uuid.UUID) (model.KYCAdminApplication, error) {
	q := store.New(s.d.DB.Conn(ctx))
	row, err := q.KYCApplicationByID(ctx, applicationID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && row.Status == string(model.KYCStatusDraft)) {
		return model.KYCAdminApplication{}, apperr.NotFound("找不到這筆申請")
	}
	if err != nil {
		return model.KYCAdminApplication{}, err
	}
	return s.adminView(ctx, q, row)
}

// adminView 組一筆申請的後台檢視（讀的人決定在哪個 transaction 內），顯示申請目前這一輪送件（submission_seq）seq 最大的已完成登記資料查詢。
func (s *Service) adminView(ctx context.Context, q *store.Queries, row store.SellerKycApplication) (model.KYCAdminApplication, error) {
	shown, err := s.latestCheck(ctx, q, row)
	if err != nil {
		return model.KYCAdminApplication{}, err
	}
	return s.adminViewWith(ctx, q, row, shown)
}

// adminViewWith 是 adminView，顯示的登記資料查詢由呼叫方給（核准與退回在寫入的同一個 transaction 內，顯示的就是記進審核紀錄
// 的那一筆，不會因為 job 在這之間寫入了更新的結果而兩者不同）。
func (s *Service) adminViewWith(ctx context.Context, q *store.Queries, row store.SellerKycApplication, shown shownCheck) (model.KYCAdminApplication, error) {
	owner, err := q.StoreByID(ctx, row.StoreID)
	if err != nil {
		return model.KYCAdminApplication{}, model.Error(err)
	}
	view, reviews, err := s.viewWithReviews(ctx, q, row, owner.AccountID)
	if err != nil {
		return model.KYCAdminApplication{}, err
	}
	st, state, err := s.reviewFacts(ctx, q, owner, row)
	if err != nil {
		return model.KYCAdminApplication{}, err
	}
	out := model.KYCAdminApplication{
		KYCApplication:     view,
		IssuableFields:     model.IssuableFields(model.KYCSellerType(row.SellerType)),
		ApproveUnavailable: model.KYCActionUnavailableReasons(st, state, model.StoreActionApproveKYC),
		RejectUnavailable:  model.KYCActionUnavailableReasons(st, state, model.StoreActionRejectKYC),
		RequeryUnavailable: model.KYCActionUnavailableReasons(st, state, model.StoreActionRequeryKYC),
	}
	if row.Status == string(model.KYCStatusCancelled) {
		// 取消的申請沒有目前的文件（取消時全部記成退場並解除引用；kyc_documents 的列還在，圖片已經不能讀）：不給文件。
		out.Documents = nil
	}
	refs, err := q.KYCReviewRegistrationChecks(ctx, row.ApplicationID)
	if err != nil {
		return model.KYCAdminApplication{}, err
	}
	for _, review := range reviews {
		if i := slices.IndexFunc(refs, func(ref store.KYCReviewRegistrationChecksRow) bool { return ref.ReviewID == review.ID }); i >= 0 {
			review.RegistrationCheckSeq, review.RegistrationCheckQueriedAt = refs[i].Seq, refs[i].QueriedAt
		}
		out.Reviews = append(out.Reviews, review)
	}
	if shown.row != nil {
		check := toRegistrationCheck(*shown.row)
		out.RegistrationCheck = &check
	}
	if row.SellerType == string(model.KYCCompany) {
		pending, err := q.PendingKYCRegistrationCheck(ctx, row.ApplicationID)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
		case err != nil:
			return model.KYCAdminApplication{}, err
		default:
			if pending.Seq > shown.seq() {
				check := toPendingCheck(pending)
				out.PendingRegistrationCheck = &check
			}
		}
	}
	return out, nil
}

// requireVisible 確認申請存在而且後台看得到（不是草稿），否則 NotFound。
func requireVisible(ctx context.Context, q *store.Queries, applicationID uuid.UUID) error {
	row, err := q.KYCApplicationByID(ctx, applicationID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && row.Status == string(model.KYCStatusDraft)) {
		return apperr.NotFound("找不到這筆申請")
	}
	return err
}

// ListDocumentAccesses 列出一筆申請的補充文件讀取紀錄，新到舊的 keyset，預設 20、上限 100；申請不存在或是草稿回 NotFound。
func (s *Service) ListDocumentAccesses(ctx context.Context, in model.KYCAuditListInput) (model.KYCDocumentAccessPage, error) {
	size, err := pagination.PageSize(in.PageSize, 20, 100)
	if err != nil {
		return model.KYCDocumentAccessPage{}, err
	}
	q := store.New(s.d.DB.Conn(ctx))
	if err := requireVisible(ctx, q, in.ApplicationID); err != nil {
		return model.KYCDocumentAccessPage{}, err
	}
	params := store.ListKYCDocumentAccessesParams{ApplicationID: in.ApplicationID, PageSize: size + 1}
	if in.After != nil {
		params.AfterAt, params.AfterID = &in.After.At, &in.After.ID
	}
	rows, err := q.ListKYCDocumentAccesses(ctx, params)
	if err != nil {
		return model.KYCDocumentAccessPage{}, err
	}
	hasMore := len(rows) > int(size)
	if hasMore {
		rows = rows[:size]
	}
	page := model.KYCDocumentAccessPage{Items: make([]model.KYCDocumentAccess, 0, len(rows))}
	for _, a := range rows {
		page.Items = append(page.Items, model.KYCDocumentAccess{
			ID: a.AccessID, AdminID: a.AdminID, Position: int(a.Position), AssetID: a.AssetID, AccessedAt: a.AccessedAt,
		})
	}
	if hasMore {
		last := page.Items[len(page.Items)-1]
		page.Next = &model.KYCAuditCursor{At: last.AccessedAt, ID: last.ID}
	}
	return page, nil
}

// ForStore 以店家讀它目前那一筆還沒取消的申請（帶補充文件，不含審核紀錄）。草稿是賣家自己的，後台讀不到：
// 沒有申請、已取消或只有草稿時 ok 為 false。
func (s *Service) ForStore(ctx context.Context, storeID int64) (view model.KYCApplication, ok bool, err error) {
	if err := model.Positive(storeID); err != nil {
		return model.KYCApplication{}, false, err
	}
	q := store.New(s.d.DB.Conn(ctx))
	owner, err := q.StoreByID(ctx, storeID)
	if err != nil {
		return model.KYCApplication{}, false, model.Error(err)
	}
	row, err := q.CurrentKYCApplication(ctx, storeID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && row.Status == string(model.KYCStatusDraft)) {
		return model.KYCApplication{}, false, nil
	}
	if err != nil {
		return model.KYCApplication{}, false, err
	}
	view, err = s.view(ctx, q, row, owner.AccountID)
	if err != nil {
		return view, false, err
	}
	one := []model.KYCApplication{view}
	if err := s.loadDeadlines(ctx, q, one); err != nil {
		return model.KYCApplication{}, false, err
	}
	return one[0], true, nil
}

// loadDeadlines 一次讀出這批申請的審核期限（DecideDeadline）。成員、開始與期限只在 seller.review_kyc_application(at)（db/objects.sql）；
// 不是審核中的申請不在函式的結果裡，期限維持 nil。
func (s *Service) loadDeadlines(ctx context.Context, q *store.Queries, apps []model.KYCApplication) error {
	if len(apps) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(apps))
	for i, a := range apps {
		ids[i] = a.ID
	}
	rows, err := q.ReviewKYCApplicationByIDs(ctx, store.ReviewKYCApplicationByIDsParams{At: s.d.Now().UTC().Truncate(time.Microsecond), Ids: ids})
	if err != nil {
		return err
	}
	byID := make(map[uuid.UUID]*model.Deadline, len(rows))
	for _, r := range rows {
		byID[r.KycApplicationID] = &model.Deadline{Start: r.StartAt, Due: r.DueAt}
	}
	for i := range apps {
		apps[i].DecideDeadline = byID[apps[i].ID]
	}
	return nil
}

// lockApplication 鎖住申請所在帳號的全部 stint（鎖順序與處分的寫入相同：先 stint.LockAccountOf），再鎖申請列。草稿是賣家自己的，
// 後台讀不到（NotFound）。
func (s *Service) lockApplication(ctx context.Context, q *store.Queries, applicationID uuid.UUID) (store.SellerStore, store.SellerKycApplication, error) {
	// store_id 不會改變，不鎖就讀是安全的。
	probe, err := q.KYCApplicationByID(ctx, applicationID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && probe.Status == string(model.KYCStatusDraft)) {
		return store.SellerStore{}, probe, apperr.NotFound("找不到這筆申請")
	}
	if err != nil {
		return store.SellerStore{}, probe, err
	}
	locked, err := stint.LockAccountOf(ctx, q, probe.StoreID)
	if err != nil {
		return locked, probe, err
	}
	row, err := q.LockKYCApplication(ctx, applicationID)
	return locked, row, err
}

// lockForReview 先 lockApplication，最後在鎖內重新確認版本、顯示的登記資料查詢與狀態（盤點 D6：審核的鎖要在 transaction 內
// 才有作用）。後台 etag 是申請的版本加顯示的查詢的序號：兩者都要與讀到的相同，否則審核員看到的比對結果不是現在的；順序是版本、
// 查詢、狀態（etag 不符一律先於狀態檢查）。回傳目前顯示的查詢（沒有已完成的查詢時 ok 為 false）：核准與退回把它記進審核紀錄。
func (s *Service) lockForReview(ctx context.Context, q *store.Queries, applicationID uuid.UUID, expectedVersion, expectedCheckSeq int64, action model.StoreAction) (store.SellerStore, store.SellerKycApplication, shownCheck, error) {
	locked, row, err := s.lockApplication(ctx, q, applicationID)
	if err != nil {
		return locked, row, shownCheck{}, err
	}
	if row.Version != expectedVersion {
		return locked, row, shownCheck{}, errVersionConflict()
	}
	shown, err := s.latestCheck(ctx, q, row)
	if err != nil {
		return locked, row, shown, err
	}
	if shown.seq() != expectedCheckSeq {
		return locked, row, shown, errVersionConflict()
	}
	reasons, err := s.reviewReasons(ctx, q, locked, row, action)
	if err != nil {
		return locked, row, shown, err
	}
	if len(reasons) > 0 {
		return locked, row, shown, model.ErrStoreActionUnavailable(reasons, action)
	}
	return locked, row, shown, nil
}

// reviewFacts 是推導核准、退回、重新查詢的原因要用的事實：店家以 stint.Load 載入（含停權）、申請的狀態與賣家型態。讀取（adminViewWith 的 can_*）與
// 方法的前置檢查（reviewReasons）都經它，事實只有一種組法。
func (s *Service) reviewFacts(ctx context.Context, q *store.Queries, owner store.SellerStore, row store.SellerKycApplication) (model.Store, model.KYCApplicationState, error) {
	st, err := stint.Load(ctx, q, owner, s.d.Now().UTC())
	if err != nil {
		return model.Store{}, model.KYCApplicationState{}, err
	}
	return st, model.KYCApplicationState{Status: model.KYCStatus(row.Status), SellerType: model.KYCSellerType(row.SellerType)}, nil
}

// reviewReasons 是不能對申請做 action 的原因（model.KYCActionUnavailableReasons），鎖已由呼叫方持有。讀取與核准、退回、重新查詢的前置檢查是同一個函式。
func (s *Service) reviewReasons(ctx context.Context, q *store.Queries, locked store.SellerStore, row store.SellerKycApplication, action model.StoreAction) ([]model.StoreActionUnavailableReason, error) {
	st, state, err := s.reviewFacts(ctx, q, locked, row)
	if err != nil {
		return nil, err
	}
	return model.KYCActionUnavailableReasons(st, state, action), nil
}

// Approve 核准審核中的申請：鎖住帳號與申請列、在鎖內重新檢查（版本、狀態、店家還在營業、申請人的帳號沒有停權、身分
// 沒有命中別的帳號生效中的處分）、設為 APPROVED、寫審核紀錄、發 KYCApproved；
// 收款帳戶「建立中」的列與建立 job 也在同一個 transaction（外部呼叫在 job，不在 transaction 內，盤點 D4、D5）。命中身分時回泛用錯誤，不透露欄位或帳號。
//
// 版本不符一律回 VersionConflict（先於狀態檢查），不寫入；不推斷重送：已經核准的申請被舊版本再核准，client 重讀後看到結果。
func (s *Service) Approve(ctx context.Context, in model.ApproveKYCInput) (model.KYCAdminApplication, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.KYCAdminApplication{}, err
	}
	comment, err := auditreason.Optional(in.Comment, "Comment")
	if err != nil {
		return model.KYCAdminApplication{}, err
	}
	var out model.KYCAdminApplication
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		locked, row, shown, err := s.lockForReview(ctx, q, in.ApplicationID, in.ExpectedVersion, in.ExpectedCheckSeq, model.StoreActionApproveKYC)
		if err != nil {
			return err
		}
		now := s.d.Now().UTC()
		blocked, err := s.match(ctx, q, row, locked.AccountID, now)
		if err != nil {
			return err
		}
		if blocked {
			return model.ErrIdentityNotEligible()
		}
		updated, err := s.transition(ctx, q, row, model.KYCStatusApproved, now)
		if err != nil {
			return err
		}
		review, err := q.InsertKYCReview(ctx, reviewParams(row.ApplicationID, model.ReviewApprove, &in.AdminID, comment, "", shown.id(), now))
		if err != nil {
			return err
		}
		var contact *string
		if s.d.Payment.Configured() {
			email, ok, err := s.contactEmail(ctx, locked.AccountID)
			if err != nil {
				return err
			}
			if !ok {
				return apperr.FailedPrecondition("賣家帳號沒有可用的聯絡信箱（登入信箱與自填信箱都沒有或格式不合），請賣家先填寫後再核准")
			}
			contact = &email
		}
		if err := s.d.Payment.RequestAccount(ctx, row.StoreID, contact, now); err != nil {
			return err
		}
		if err := s.d.Exposure.Announce(ctx, q, now, row.StoreID); err != nil {
			return err
		}
		err = events.Publish(ctx, s.d.Events, model.KYCApproved{
			EventID: review.ReviewID, OccurredAt: now, ApplicationID: row.ApplicationID, StoreID: row.StoreID, AccountID: locked.AccountID,
		})
		if err != nil {
			return err
		}
		out, err = s.adminViewWith(ctx, q, updated, shown)
		return err
	})
	return out, err
}

// Reject 退回審核中的申請：Reason 必填，Issues 是逐項指出的問題（至少一項，欄位與文件必須是這份申請實際有的），InternalNote 是只給
// 後台看的備註。鎖與重新檢查同 Approve；寫審核紀錄與問題、發 KYCRejected。賣家修改後重送沿用同一筆申請。
func (s *Service) Reject(ctx context.Context, in model.RejectKYCInput) (model.KYCAdminApplication, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.KYCAdminApplication{}, err
	}
	reason, err := auditreason.Required(in.Reason, "Reason")
	if err != nil {
		return model.KYCAdminApplication{}, err
	}
	note, err := auditreason.Optional(in.InternalNote, "InternalNote")
	if err != nil {
		return model.KYCAdminApplication{}, err
	}
	issues, err := validateIssues(in.Issues)
	if err != nil {
		return model.KYCAdminApplication{}, err
	}
	var out model.KYCAdminApplication
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		locked, row, shown, err := s.lockForReview(ctx, q, in.ApplicationID, in.ExpectedVersion, in.ExpectedCheckSeq, model.StoreActionRejectKYC)
		if err != nil {
			return err
		}
		docs, err := q.KYCDocuments(ctx, row.ApplicationID)
		if err != nil {
			return err
		}
		if err := checkIssuesAgainstApplication(issues, model.KYCSellerType(row.SellerType), docs); err != nil {
			return err
		}
		now := s.d.Now().UTC()
		updated, err := s.transition(ctx, q, row, model.KYCStatusRejected, now)
		if err != nil {
			return err
		}
		review, err := q.InsertKYCReview(ctx, reviewParams(row.ApplicationID, model.ReviewReject, &in.AdminID, reason, note, shown.id(), now))
		if err != nil {
			return err
		}
		if err := insertIssues(ctx, q, review.ReviewID, issues); err != nil {
			return err
		}
		err = events.Publish(ctx, s.d.Events, model.KYCRejected{
			EventID: review.ReviewID, OccurredAt: now, ApplicationID: row.ApplicationID, StoreID: row.StoreID,
			AccountID: locked.AccountID, Reason: reason,
		})
		if err != nil {
			return err
		}
		if err := s.d.Exposure.Announce(ctx, q, now, row.StoreID); err != nil {
			return err
		}
		out, err = s.adminViewWith(ctx, q, updated, shown)
		return err
	})
	return out, err
}

// ReadDocument 讀一張公司補充文件。授權是後台的 KYC 讀取權限（路由的 adminperm.SellerKYCDocumentsGet，由 httpapi 檢查；Stripe 計畫 D36：補充文件不是證件，
// 不要理由與限時授權），這裡不再問授權；個資已清除、草稿與已取消的申請或位置沒有文件是 NotFound。
// 順序是安全性質：（1）在 transaction 內確認文件並寫一列 append-only 的讀取紀錄（誰、哪個位置、實際送出的 asset ID、
// 何時；不含內容），commit 之後才（2）向 media 讀圖片。紀錄寫不進去就不讀（fail closed）；讀取失敗時紀錄仍在（嘗試也是紀錄）。
// media 的呼叫在 transaction 之外，讀不到時的三種結果見 mediaref.ReadSecure。
func (s *Service) ReadDocument(ctx context.Context, in model.ReadKYCDocumentInput) (model.FileContent, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.FileContent{}, err
	}
	if in.Position < 0 || in.Position >= model.MaxCompanyDocuments {
		return model.FileContent{}, apperr.NotFound("找不到這份文件")
	}
	var doc store.SellerKycDocument
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		now := s.d.Now().UTC()
		row, err := q.KYCApplicationByID(ctx, in.ApplicationID)
		// 個資已清除與已取消的申請（文件已記成退場、引用已解除，列還在）沒有可讀的文件：不寫讀取紀錄（只有讀得到的才記）。
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && (row.PiiPurgedAt != nil || row.Status == string(model.KYCStatusDraft) || row.Status == string(model.KYCStatusCancelled))) {
			return apperr.NotFound("找不到這份文件")
		}
		if err != nil {
			return err
		}
		docs, err := q.KYCDocuments(ctx, in.ApplicationID)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(docs, func(d store.SellerKycDocument) bool { return int(d.Position) == in.Position })
		if i < 0 {
			return apperr.NotFound("找不到這份文件")
		}
		doc = docs[i]
		_, err = q.InsertKYCDocumentAccess(ctx, store.InsertKYCDocumentAccessParams{
			AccessID: uuid.NewV7(), ApplicationID: in.ApplicationID, Position: int32(in.Position), //nolint:gosec // position < 10
			AssetID: doc.AssetID, AdminID: in.AdminID, Now: now,
		})
		return err
	})
	if err != nil {
		return model.FileContent{}, err
	}
	return mediaref.ReadSecure(ctx, store.New(s.d.DB.Conn(ctx)), s.d.Media, media.Ref{Type: media.RefTypeCertification, ID: in.ApplicationID.String()}, doc.ObjectKey)
}

// MatchCertHolder 回報 holderName（政府證書查驗回的業者名稱）是否屬於店家目前已核准的 KYC 的公司名稱（規則在
// model.MatchCertHolder，名字不出這個函式）。基準只有目前的 stint 上人工核准（KYC APPROVED）的申請的公司名稱（`docs/modules/seller.md` §1.3；
// 收款帳戶啟用是金流開通，不是身分驗證，所以不是條件）：店家不存在、已關閉、沒有申請、申請不是 APPROVED、個人賣家（沒有公司名稱；Stripe 計畫 D37：平台不再收
// 個人姓名）都是沒有基準（HasReference=false，呼叫方轉人工），不是錯誤；讀取失敗回錯誤，呼叫方必須 fail-closed。
func (s *Service) MatchCertHolder(ctx context.Context, storeID int64, holderName string) (model.CertHolderMatch, error) {
	if err := model.Positive(storeID); err != nil {
		return model.CertHolderMatch{}, err
	}
	none := model.CertHolderMatch{Field: model.CertHolderNone}
	q := store.New(s.d.DB.Conn(ctx))
	owner, err := q.StoreByID(ctx, storeID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && owner.Status == string(model.Closed)) {
		return none, nil
	}
	if err != nil {
		return model.CertHolderMatch{}, err
	}
	row, err := q.CurrentKYCApplication(ctx, storeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return none, nil
	}
	if err != nil {
		return model.CertHolderMatch{}, err
	}
	if row.Status != string(model.KYCStatusApproved) {
		return none, nil
	}
	return model.MatchCertHolder(holderName, row.CompanyName), nil
}
