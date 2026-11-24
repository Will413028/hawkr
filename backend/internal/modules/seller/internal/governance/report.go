package governance

import (
	"context"
	"errors"
	"slices"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// validateSubmit 一次驗證全部輸入，回傳寫入的欄位（ID 與時間由呼叫方補上）。
func validateSubmit(in *model.SubmitReportInput) (store.InsertReportParams, error) {
	params := store.InsertReportParams{StoreID: in.StoreID, Category: string(in.Category)}
	if err := model.Positive(in.StoreID); err != nil {
		return params, err
	}
	if err := model.OneOf(in.Category, policy.ReporterSelectable(), "Category", "通報類別不合法"); err != nil {
		return params, err
	}
	var err error
	if params.Description, err = model.Required(in.Description, 2000, "Description", "通報內容"); err != nil {
		return params, err
	}
	if params.ProductID, err = model.Optional(in.ProductID, 64, "ProductID", "商品編號"); err != nil {
		return params, err
	}
	if params.OrderNo, err = model.Optional(in.OrderNo, 64, "OrderNo", "訂單編號"); err != nil {
		return params, err
	}
	if err := model.CheckEvidenceTokens(in.EvidenceTokens); err != nil {
		return params, err
	}
	if in.ReporterBuyerID <= 0 {
		return params, apperr.InvalidInputOn([]string{"ReporterBuyerID"}, "ID 必須為正數")
	}
	if len(in.EvidenceTokens) > 0 && in.AccountID <= 0 {
		return params, apperr.InvalidInputOn([]string{"AccountID"}, "附佐證時必須提供上傳者")
	}
	params.ReporterBuyerID = in.ReporterBuyerID
	return params, nil
}

// Submit 提出檢舉（登入的買家）。店家必須存在（已關閉的也可以檢舉）；呼叫者登入（AccountID）而且店家屬於他的帳號時回 invalid_argument（StoreID）。
// 去重的期間內（同買家、同店家、同類別、同一個台北日）已有還沒有結果的通報時，這次的內容補充到它
// （Supplemented）：每筆通報最多 MaxSupplements 則，已達上限時回 failed_precondition。每次送出都在同一個
// transaction 發 ReportReceived（新的檢舉或補充）。已有結果的通報不再比對：這次是新的檢舉。佐證在寫入的 transaction 內 Confirm（只 Confirm，從不 MarkDeleted），任何一步失敗整筆回滾。
func (s *Service) Submit(ctx context.Context, in model.SubmitReportInput) (model.SubmitReportResult, error) {
	params, err := validateSubmit(&in)
	if err != nil {
		return model.SubmitReportResult{}, err
	}
	now := s.now()
	target, err := store.New(s.d.DB.Conn(ctx)).StoreByID(ctx, in.StoreID)
	if err != nil {
		return model.SubmitReportResult{}, model.Error(err)
	}
	if in.AccountID > 0 && target.AccountID == in.AccountID {
		return model.SubmitReportResult{}, invalidOn("StoreID", text.ReportOwnStore(uitext.LocaleFrom(ctx)))
	}
	params.ID, params.Now = uuid.NewV7(), now
	var result model.SubmitReportResult
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		evidence, err := model.ConfirmEvidence(ctx, s.d.Media, media.Uploader{ID: in.AccountID}, in.EvidenceTokens)
		if err != nil {
			return err
		}
		result, err = s.submit(ctx, store.New(s.d.DB.Conn(ctx)), params, in, evidence, now)
		return err
	})
	if err != nil {
		return model.SubmitReportResult{}, err
	}
	return result, nil
}

// submitAttempts 是一次送出寫入新通報的次數：撞到去重的那一筆在鎖住時已經有結果（剛被結案）就重做一次，那時它已離開 partial UNIQUE。
const submitAttempts = 2

// submit 在 transaction 內寫入一次送出：新通報或補充。撞到去重的 InsertReport 是 ON CONFLICT DO NOTHING（0 列），
// 不會讓 transaction 失效，之後的 SQL 照常執行。
func (s *Service) submit(ctx context.Context, q *store.Queries, params store.InsertReportParams, in model.SubmitReportInput, evidence []model.Evidence, now time.Time) (model.SubmitReportResult, error) {
	for range submitAttempts {
		row, err := q.InsertReport(ctx, params)
		if err == nil {
			return s.created(ctx, q, row, evidence, now)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return model.SubmitReportResult{}, err
		}
		existing, err := s.duplicate(ctx, q, params, now)
		if errors.Is(err, pgx.ErrNoRows) {
			// 撞到的那一筆在兩次查詢之間有了結果，已經離開去重的範圍。
			continue
		}
		if err != nil {
			return model.SubmitReportResult{}, err
		}
		locked, err := q.LockReport(ctx, existing.ID)
		if err != nil {
			return model.SubmitReportResult{}, err
		}
		if locked.Outcome != nil {
			// 等鎖的時候它被結案了：這次是新的檢舉。
			continue
		}
		return s.supplement(ctx, q, locked, params, evidence, now)
	}
	return model.SubmitReportResult{}, apperr.Unavailable("%s", text.ReportSubmitConflict(uitext.LocaleFrom(ctx)).Text)
}

// created 寫入新通報的佐證、排入引用的同步並發 ReportReceived。
func (s *Service) created(ctx context.Context, q *store.Queries, row store.SellerReport, evidence []model.Evidence, now time.Time) (model.SubmitReportResult, error) {
	if err := s.writeEvidence(ctx, q, row.ID, nil, 0, evidence); err != nil {
		return model.SubmitReportResult{}, err
	}
	report := toReport(row)
	report.Evidence = evidence
	if err := events.Publish(ctx, s.d.Events, model.ReportReceived{
		EventID: uuid.NewV7(), OccurredAt: now, ReportID: row.ID, StoreID: row.StoreID, ReporterBuyerID: row.ReporterBuyerID,
		Category: report.Category,
	}); err != nil {
		return model.SubmitReportResult{}, err
	}
	return model.SubmitReportResult{ReportID: row.ID, Report: &report}, nil
}

// supplement 把這次的送出併入已鎖住、還沒有結果的原通報。
func (s *Service) supplement(ctx context.Context, q *store.Queries, locked store.SellerReport, params store.InsertReportParams, evidence []model.Evidence, now time.Time) (model.SubmitReportResult, error) {
	if err := s.appendSupplement(ctx, q, locked, params.Description, evidence, now); err != nil {
		return model.SubmitReportResult{}, err
	}
	// 去重的鍵含買家，所以那一筆一定是呼叫者自己的：回應用它，並告訴他發生了什麼。
	row, err := q.ReportByID(ctx, locked.ID)
	if err != nil {
		return model.SubmitReportResult{}, err
	}
	report := toReport(row)
	count, err := q.SupplementCount(ctx, locked.ID)
	if err != nil {
		return model.SubmitReportResult{}, err
	}
	report.SupplementCount = int(count)
	return model.SubmitReportResult{ReportID: locked.ID, Supplemented: true, Report: &report, Notice: text.ReportDuplicateSupplemented(uitext.LocaleFrom(ctx))}, nil
}

// supplementBlockError 是 model.SupplementBlockOf 的結果對應的 failed_precondition；可以補充時回 nil。
func supplementBlockError(ctx context.Context, b model.SupplementBlock) error {
	switch b {
	case model.SupplementBlockedByOutcome:
		return apperr.FailedPrecondition("%s", text.ReportSupplementClosed(uitext.LocaleFrom(ctx)).Text)
	case model.SupplementBlockedByLimit:
		return apperr.FailedPrecondition("%s", text.ReportSupplementLimit(uitext.LocaleFrom(ctx), text.ReportSupplementLimitArgs{Max: model.MaxSupplements}).Text)
	case model.SupplementAllowed:
	}
	return nil
}

// appendSupplement 是補充的唯一寫入路徑（去重併入與檢舉人主動補充共用）：通報已鎖住；有結果或合計已達 MaxSupplements 則時回 failed_precondition（model.SupplementBlockOf）。
// 寫入補充與佐證、讓通報的版本前進（讀到舊版本的分案、設定結果與移動要重讀才看得到補充；不鎖、不改案件的版本），並在同一個 transaction 發帶 SupplementID 的 ReportReceived。
func (s *Service) appendSupplement(ctx context.Context, q *store.Queries, locked store.SellerReport, description string, evidence []model.Evidence, now time.Time) error {
	count, err := q.SupplementCount(ctx, locked.ID)
	if err != nil {
		return err
	}
	received := model.ReportReceived{
		EventID: uuid.NewV7(), OccurredAt: now, ReportID: locked.ID, StoreID: locked.StoreID, ReporterBuyerID: locked.ReporterBuyerID,
		Category: policy.Category(locked.Category),
	}
	if err := supplementBlockError(ctx, model.SupplementBlockOf(locked.Outcome != nil, int(count))); err != nil {
		return err
	}
	supplementID := uuid.NewV7()
	// 第幾則補充 = 已有的則數 + 1（通報已鎖住，所以不會撞；DB 的 seq 1..10 與每筆通報內唯一是保底）。
	seq := int16(count) + 1 //nolint:gosec // count < MaxSupplements
	if _, err := q.InsertReportSupplement(ctx, store.InsertReportSupplementParams{
		ID: supplementID, ReportID: locked.ID, Seq: seq, Description: description, Now: now,
	}); err != nil {
		return err
	}
	if err := s.writeEvidence(ctx, q, locked.ID, &supplementID, seq, evidence); err != nil {
		return err
	}
	if err := q.BumpReportVersion(ctx, store.BumpReportVersionParams{ID: locked.ID, Now: now}); err != nil {
		return err
	}
	received.SupplementID = &supplementID
	return events.Publish(ctx, s.d.Events, received)
}

// AddSupplement 是檢舉人為自己還沒有結果的通報主動補充。別人的或不存在的通報一律 NotFound；通報已有結果回
// failed_precondition；寫入走 appendSupplement，與去重併入的補充同一串、同一個上限。回傳補充之後的通報（含全部補充與佐證）。
func (s *Service) AddSupplement(ctx context.Context, in model.AddSupplementInput) (model.Report, error) {
	if in.BuyerID <= 0 {
		return model.Report{}, apperr.InvalidInputOn([]string{"BuyerID"}, "ID 必須為正數")
	}
	description, err := model.Required(in.Description, 2000, "Description", "補充內容")
	if err != nil {
		return model.Report{}, err
	}
	if err := model.CheckEvidenceTokens(in.EvidenceTokens); err != nil {
		return model.Report{}, err
	}
	if len(in.EvidenceTokens) > 0 && in.AccountID <= 0 {
		return model.Report{}, apperr.InvalidInputOn([]string{"AccountID"}, "附佐證時必須提供上傳者")
	}
	// 佐證轉正之前先確認通報是呼叫者的，別人的 id 不會讓呼叫者的檔被轉正。
	owned, err := store.New(s.d.DB.Conn(ctx)).ReportByID(ctx, in.ReportID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && owned.ReporterBuyerID != in.BuyerID) {
		return model.Report{}, errMyReportNotFound(ctx)
	}
	if err != nil {
		return model.Report{}, err
	}
	// 佐證轉正之前先擋下（有結果或已達上限）；寫入時鎖住之後在 appendSupplement 再判斷一次。
	count, err := store.New(s.d.DB.Conn(ctx)).SupplementCount(ctx, in.ReportID)
	if err != nil {
		return model.Report{}, err
	}
	if err := supplementBlockError(ctx, model.SupplementBlockOf(owned.Outcome != nil, int(count))); err != nil {
		return model.Report{}, err
	}
	now := s.now()
	var out model.Report
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		locked, err := q.LockReport(ctx, in.ReportID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && locked.ReporterBuyerID != in.BuyerID) {
			return errMyReportNotFound(ctx)
		}
		if err != nil {
			return err
		}
		evidence, err := model.ConfirmEvidence(ctx, s.d.Media, media.Uploader{ID: in.AccountID}, in.EvidenceTokens)
		if err != nil {
			return err
		}
		if err := s.appendSupplement(ctx, q, locked, description, evidence, now); err != nil {
			return err
		}
		out, err = loadBuyerReport(ctx, q, in.ReportID)
		return err
	})
	return out, err
}

func errMyReportNotFound(ctx context.Context) error {
	return apperr.NotFound("%s", text.ReportMyNotFound(uitext.LocaleFrom(ctx)).Text)
}

// BuyerReport 讀檢舉人自己的一筆通報（含補充與佐證）；別人的或不存在的是 NotFound。
func (s *Service) BuyerReport(ctx context.Context, buyerID int64, reportID uuid.UUID) (model.Report, error) {
	if err := model.Positive(buyerID); err != nil {
		return model.Report{}, err
	}
	var out model.Report
	err := s.d.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		row, err := q.ReportByID(ctx, reportID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && row.ReporterBuyerID != buyerID) {
			return errMyReportNotFound(ctx)
		}
		if err != nil {
			return err
		}
		out, err = loadBuyerReport(ctx, q, reportID)
		return err
	})
	return out, err
}

// loadBuyerReport 讀一筆通報的內容、補充與佐證（不做擁有者檢查，呼叫方已確認）。
func loadBuyerReport(ctx context.Context, q *store.Queries, reportID uuid.UUID) (model.Report, error) {
	row, err := q.ReportByID(ctx, reportID)
	if err != nil {
		return model.Report{}, err
	}
	reports := []model.Report{toReport(row)}
	if err := loadReportDetail(ctx, q, []uuid.UUID{reportID}, reports); err != nil {
		return model.Report{}, err
	}
	reports[0].SupplementCount = len(reports[0].Supplements)
	return reports[0], nil
}

// writeEvidence 寫入一次送出（submissionSeq：0 是原通報、1.. 是第幾則補充）的佐證，次內位置從 0 連號（DB 的 CHECK 保證最多 5 張），補充的佐證帶 supplementID；
func (s *Service) writeEvidence(ctx context.Context, q *store.Queries, reportID uuid.UUID, supplementID *uuid.UUID, submissionSeq int16, evidence []model.Evidence) error {
	for i, e := range evidence {
		if err := q.InsertReportEvidence(ctx, store.InsertReportEvidenceParams{
			ReportID: reportID, SubmissionSeq: submissionSeq, Position: int32(i), SupplementID: supplementID, AssetID: e.AssetID, ObjectKey: e.ObjectKey, //nolint:gosec // 位置最多 MaxEvidence
		}); err != nil {
			return err
		}
	}
	return nil
}

// duplicate 讀出去重的期間內同一個買家已有、還沒有結果的通報（與 DB 的 partial UNIQUE 同一個條件）。沒有時回 pgx.ErrNoRows。
func (s *Service) duplicate(ctx context.Context, q *store.Queries, params store.InsertReportParams, now time.Time) (store.SellerReport, error) {
	return q.DuplicateBuyerReport(ctx, store.DuplicateBuyerReportParams{
		BuyerID: params.ReporterBuyerID, StoreID: params.StoreID, Category: params.Category, Now: now,
	})
}

// Report 讀取一筆通報的後台檢視：補充、佐證、此刻可以做的動作與 etag（Version）。
func (s *Service) Report(ctx context.Context, reportID uuid.UUID) (model.AdminReport, error) {
	var out model.AdminReport
	err := s.d.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		var err error
		out, err = readReport(ctx, store.New(s.d.DB.Conn(ctx)), reportID, s.now())
		return err
	})
	return out, err
}

// AdminReports 是 admin 的通報清單：可依處理狀態、生效的類別（案件裡的是平台判定類別，其他是檢舉人選的類別）、店家、案件與「可以當認定的來源」篩選。預設新到舊；待分案與案件的成員
// 舊到新（等最久的在最前面）。清單與單筆的 can_* 由同一個 Availability；每頁的查詢數固定，與頁大小無關。
func (s *Service) AdminReports(ctx context.Context, in model.ReportListInput) (model.AdminReportPage, error) {
	size, err := pageSize(in.PageSize)
	if err != nil {
		return model.AdminReportPage{}, err
	}
	if in.State != nil {
		if err := model.OneOf(*in.State, model.ReportStateValues(), "State", text.ListStateInvalid().Text); err != nil {
			return model.AdminReportPage{}, err
		}
	}
	if in.Category != nil {
		if err := model.OneOf(*in.Category, reportFilterCategories(), "Category", text.ListCategoryInvalid().Text); err != nil {
			return model.AdminReportPage{}, err
		}
	}
	if in.StoreID < 0 {
		return model.AdminReportPage{}, model.Positive(in.StoreID)
	}
	var state, category *string
	if in.State != nil {
		state = (*string)(in.State)
	}
	if in.Category != nil {
		category = (*string)(in.Category)
	}
	var storeID *int64
	if in.StoreID > 0 {
		storeID = &in.StoreID
	}
	var page model.AdminReportPage
	err = s.d.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		var rows []reportRow
		if in.OldestFirst() {
			params := store.AdminReportsQueueParams{State: state, Category: category, StoreID: storeID, CaseID: in.CaseID, CitableAsSource: in.CitableAsSource, PageSize: size + 1}
			if in.After != nil {
				params.AfterCreatedAt, params.AfterID = &in.After.CreatedAt, &in.After.ID
			}
			found, err := q.AdminReportsQueue(ctx, params)
			if err != nil {
				return err
			}
			for _, r := range found {
				rows = append(rows, reportRow{report: r.SellerReport, storeName: r.StoreName, storeClosed: r.StoreClosed, supplementCount: r.SupplementCount, latestSupplementSeq: r.LatestSupplementSeq, latestSupplementAt: timeOf(r.LatestSupplementAt)})
			}
		} else {
			params := store.AdminReportsParams{State: state, Category: category, StoreID: storeID, CaseID: in.CaseID, CitableAsSource: in.CitableAsSource, PageSize: size + 1}
			if in.After != nil {
				params.AfterCreatedAt, params.AfterID = &in.After.CreatedAt, &in.After.ID
			}
			found, err := q.AdminReports(ctx, params)
			if err != nil {
				return err
			}
			for _, r := range found {
				rows = append(rows, reportRow{report: r.SellerReport, storeName: r.StoreName, storeClosed: r.StoreClosed, supplementCount: r.SupplementCount, latestSupplementSeq: r.LatestSupplementSeq, latestSupplementAt: timeOf(r.LatestSupplementAt)})
			}
		}
		more := len(rows) > int(size)
		if more {
			rows = rows[:size]
		}
		page.Items, err = adminReports(ctx, q, rows, false, s.now())
		if err != nil {
			return err
		}
		if more {
			last := page.Items[size-1]
			page.Next = &model.ReportCursor{CreatedAt: last.CreatedAt, ID: last.ID}
		}
		return nil
	})
	return page, err
}

// BuyerReports 是登入的檢舉人自己的通報，新到舊。
func (s *Service) BuyerReports(ctx context.Context, in model.BuyerReportListInput) (model.ReportPage, error) {
	if err := model.Positive(in.BuyerID); err != nil {
		return model.ReportPage{}, err
	}
	size, err := pageSize(in.PageSize)
	if err != nil {
		return model.ReportPage{}, err
	}
	params := store.BuyerReportsParams{BuyerID: in.BuyerID, PageSize: size + 1}
	if in.After != nil {
		params.AfterCreatedAt, params.AfterID = &in.After.CreatedAt, &in.After.ID
	}
	q := store.New(s.d.DB.Conn(ctx))
	rows, err := q.BuyerReports(ctx, params)
	if err != nil {
		return model.ReportPage{}, err
	}
	more := len(rows) > int(size)
	if more {
		rows = rows[:size]
	}
	ids := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	counts, err := q.SupplementCounts(ctx, ids)
	if err != nil {
		return model.ReportPage{}, err
	}
	byReport := make(map[uuid.UUID]int, len(counts))
	for _, c := range counts {
		byReport[c.ReportID] = int(c.SupplementCount)
	}
	page := model.ReportPage{Items: make([]model.Report, 0, len(rows))}
	for _, row := range rows {
		r := toReport(row)
		r.SupplementCount = byReport[row.ID]
		page.Items = append(page.Items, r)
	}
	if more {
		last := page.Items[size-1]
		page.Next = &model.ReportCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return page, nil
}

// reportFilterCategories 是通報清單可以篩選的生效類別：檢舉人選的（ReporterSelectable）加平台判定的（Assessable），也就是除了社群類別
// 以外的全部；社群類別只給 social 的隱藏與警告，通報不會有。
func reportFilterCategories() []policy.Category {
	return slices.DeleteFunc(policy.Values(), policy.Category.Community)
}
