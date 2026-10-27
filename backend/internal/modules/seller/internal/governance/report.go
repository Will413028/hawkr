package governance

import (
	"context"
	"errors"
	"net/mail"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/phone"
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
	if err := mediaref.EvidenceTokens(in.EvidenceTokens); err != nil {
		return params, err
	}
	switch {
	case in.ReporterBuyerID < 0:
		return params, apperr.InvalidInputOn([]string{"ReporterBuyerID"}, "ID 必須為正數")
	case in.ReporterBuyerID > 0:
		if strings.TrimSpace(in.Phone+in.Email+in.Name) != "" {
			return params, apperr.InvalidInputOn([]string{"Phone", "Email", "Name"}, "登入的通報人不留聯絡方式")
		}
		if len(in.EvidenceTokens) > 0 && in.AccountID <= 0 {
			return params, apperr.InvalidInputOn([]string{"AccountID"}, "附佐證時必須提供上傳者")
		}
		params.ReporterBuyerID = &in.ReporterBuyerID
		return params, nil
	}
	if len(in.EvidenceTokens) > 0 {
		return params, apperr.InvalidInputOn([]string{"EvidenceTokens"}, "匿名通報不收佐證，請登入後通報")
	}
	if p := model.Text(in.Phone); p != nil {
		normalized := phone.NormalizeToLocal(*p)
		if !phone.Valid(normalized) {
			return params, apperr.InvalidInputOn([]string{"Phone"}, "聯絡電話不合法")
		}
		params.ReporterPhone = &normalized
	}
	if e := model.Text(in.Email); e != nil {
		normalized := strings.ToLower(*e)
		if address, err := mail.ParseAddress(normalized); err != nil || address.Address != normalized || len(normalized) > 254 {
			return params, apperr.InvalidInputOn([]string{"Email"}, "email 格式不正確")
		}
		params.ReporterEmail = &normalized
	}
	if params.ReporterPhone == nil && params.ReporterEmail == nil {
		return params, apperr.InvalidInputOn([]string{"Phone", "Email"}, "匿名通報必須留下電話或 email")
	}
	if params.ReporterName, err = model.Optional(in.Name, 100, "Name", "姓名"); err != nil {
		return params, err
	}
	return params, nil
}

// Submit 提出檢舉（登入的買家或匿名者）。店家必須存在（已關閉的也可以檢舉）；呼叫者登入（AccountID）而且店家屬於他的帳號時回 invalid_argument（StoreID）。
// 去重的期間內（登入：同店家、同類別、同一個台北日；匿名：同電話或 email、同店家、同一個台北 ISO 週）已有還沒有結果的通報時，這次的內容補充到它
// （Supplemented）：每筆通報最多 MaxSupplements 則，已達上限時登入的回 failed_precondition，匿名的內容不存、只計次數（Discarded）。每次送出都在同一個
// transaction 發 ReportReceived（新的檢舉、補充、被丟掉的），確認信寄到這次送出留的 email，所以送出的人分辨不出是哪一種。已有結果的通報不再比對：
// 這次是新的檢舉。佐證依 media 的順序在 transaction 外先轉正，以通報 ID 登記引用；寫入失敗時排入引用的收斂。
func (s *Service) Submit(ctx context.Context, in model.SubmitReportInput) (model.SubmitReportResult, error) {
	params, err := validateSubmit(&in)
	if err != nil {
		return model.SubmitReportResult{}, err
	}
	if _, ok := db.TxFrom(ctx); ok {
		return model.SubmitReportResult{}, apperr.FailedPrecondition("通報必須在 transaction 外開始")
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
	evidence, err := mediaref.FinalizeEvidence(ctx, s.d.Media, media.Uploader{ID: in.AccountID}, in.EvidenceTokens)
	if err != nil {
		return model.SubmitReportResult{}, err
	}
	var result model.SubmitReportResult
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		var err error
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

// submit 在 transaction 內寫入一次送出：新通報、補充，或超過上限而丟掉的匿名送出。撞到去重的 InsertReport 是 ON CONFLICT DO NOTHING（0 列），
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
		existing, matchedBy, err := s.duplicate(ctx, q, params, now)
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
		return s.supplement(ctx, q, locked, matchedBy, params, in, evidence, now)
	}
	return model.SubmitReportResult{}, apperr.Unavailable("%s", text.ReportSubmitConflict(uitext.LocaleFrom(ctx)).Text)
}

// created 寫入新通報的佐證、排入引用的同步並發 ReportReceived。
func (s *Service) created(ctx context.Context, q *store.Queries, row store.SellerReport, evidence []model.Evidence, now time.Time) (model.SubmitReportResult, error) {
	if err := s.writeEvidence(ctx, q, row.ID, nil, 0, evidence, now); err != nil {
		return model.SubmitReportResult{}, err
	}
	report := toReport(row)
	report.Evidence = evidence
	if err := events.Publish(ctx, s.d.Events, model.ReportReceived{
		EventID: uuid.NewV7(), OccurredAt: now, ReportID: row.ID, StoreID: row.StoreID, ReporterBuyerID: row.ReporterBuyerID,
		Category: report.Category, ConfirmationEmail: row.ReporterEmail,
	}); err != nil {
		return model.SubmitReportResult{}, err
	}
	return model.SubmitReportResult{ReportID: row.ID, Report: &report}, nil
}

// supplement 把這次的送出併入已鎖住、還沒有結果的原通報。
func (s *Service) supplement(ctx context.Context, q *store.Queries, locked store.SellerReport, matchedBy *model.SupplementMatch, params store.InsertReportParams, in model.SubmitReportInput, evidence []model.Evidence, now time.Time) (model.SubmitReportResult, error) {
	count, err := q.SupplementCount(ctx, locked.ID)
	if err != nil {
		return model.SubmitReportResult{}, err
	}
	received := model.ReportReceived{
		EventID: uuid.NewV7(), OccurredAt: now, ReportID: locked.ID, StoreID: locked.StoreID, ReporterBuyerID: locked.ReporterBuyerID,
		Category: policy.Category(locked.Category), ConfirmationEmail: params.ReporterEmail,
	}
	if int(count) >= model.MaxSupplements {
		if params.ReporterBuyerID != nil {
			// 已轉正的佐證沒有引用，由 media 的 GC 回收（沿用撞到重複時的先例）。
			return model.SubmitReportResult{}, apperr.FailedPrecondition("%s", text.ReportSupplementLimit(uitext.LocaleFrom(ctx), text.ReportSupplementLimitArgs{Max: model.MaxSupplements}).Text)
		}
		if err := q.IncrementDiscardedSubmissions(ctx, store.IncrementDiscardedSubmissionsParams{ID: locked.ID, Now: now}); err != nil {
			return model.SubmitReportResult{}, err
		}
		received.Discarded = true
		if err := events.Publish(ctx, s.d.Events, received); err != nil {
			return model.SubmitReportResult{}, err
		}
		return model.SubmitReportResult{ReportID: locked.ID, Discarded: true}, nil
	}
	supplementID := uuid.NewV7()
	var matched *string
	if matchedBy != nil {
		m := string(*matchedBy)
		matched = &m
	}
	// 第幾則補充 = 已有的則數 + 1（通報已鎖住，所以不會撞；DB 的 seq 1..10 與每筆通報內唯一是保底）。
	seq := int16(count) + 1 //nolint:gosec // count < MaxSupplements
	if _, err := q.InsertReportSupplement(ctx, store.InsertReportSupplementParams{
		ID: supplementID, ReportID: locked.ID, Seq: seq, Description: params.Description, MatchedBy: matched, Now: now,
	}); err != nil {
		return model.SubmitReportResult{}, err
	}
	if err := s.writeEvidence(ctx, q, locked.ID, &supplementID, seq, evidence, now); err != nil {
		return model.SubmitReportResult{}, err
	}
	// 補充讓通報的版本前進（讀到舊版本的分案、設定結果與移動要重讀才看得到補充）；不鎖、不改案件（補充不改案件的版本）。
	if err := q.BumpReportVersion(ctx, store.BumpReportVersionParams{ID: locked.ID, Now: now}); err != nil {
		return model.SubmitReportResult{}, err
	}
	received.SupplementID = &supplementID
	if err := events.Publish(ctx, s.d.Events, received); err != nil {
		return model.SubmitReportResult{}, err
	}
	result := model.SubmitReportResult{ReportID: locked.ID, Supplemented: true}
	if params.ReporterBuyerID != nil {
		// 去重的鍵含買家，所以那一筆一定是呼叫者自己的：回應用它，並告訴他發生了什麼。匿名的什麼都不回（重複提交的人不一定是原本的檢舉人）。
		row, err := q.ReportByID(ctx, locked.ID)
		if err != nil {
			return model.SubmitReportResult{}, err
		}
		report := toReport(row)
		result.Report, result.Notice = &report, text.ReportDuplicateSupplemented(uitext.LocaleFrom(ctx))
	}
	return result, nil
}

// writeEvidence 寫入一次送出（submissionSeq：0 是原通報、1.. 是第幾則補充）的佐證，次內位置從 0 連號（DB 的 CHECK 保證最多 5 張），補充的佐證帶 supplementID；
// 並排入引用的同步（ref 仍是通報 ID）。
func (s *Service) writeEvidence(ctx context.Context, q *store.Queries, reportID uuid.UUID, supplementID *uuid.UUID, submissionSeq int16, evidence []model.Evidence, now time.Time) error {
	for i, e := range evidence {
		if err := q.InsertReportEvidence(ctx, store.InsertReportEvidenceParams{
			ReportID: reportID, SubmissionSeq: submissionSeq, Position: int32(i), SupplementID: supplementID, AssetID: e.AssetID, ObjectKey: e.ObjectKey, //nolint:gosec // 位置最多 MaxEvidence
		}); err != nil {
			return err
		}
	}
	if len(evidence) == 0 {
		return nil
	}
	return mediaref.Changed(ctx, q, s.d.Jobs, now, mediaref.EvidenceRef(reportID))
}

// duplicate 讀出去重的期間內同一個檢舉人已有、還沒有結果的通報（與 DB 的 partial UNIQUE 同一個條件）；匿名的另回比對方式（電話、聯絡信箱或兩者）。
// 沒有時回 pgx.ErrNoRows。
func (s *Service) duplicate(ctx context.Context, q *store.Queries, params store.InsertReportParams, now time.Time) (store.SellerReport, *model.SupplementMatch, error) {
	if params.ReporterBuyerID != nil {
		row, err := q.DuplicateBuyerReport(ctx, store.DuplicateBuyerReportParams{
			BuyerID: *params.ReporterBuyerID, StoreID: params.StoreID, Category: params.Category, Now: now,
		})
		return row, nil, err
	}
	row, err := q.DuplicateContactReport(ctx, store.DuplicateContactReportParams{
		StoreID: params.StoreID, Phone: params.ReporterPhone, Email: params.ReporterEmail, Now: now,
	})
	if err != nil {
		return store.SellerReport{}, nil, err
	}
	matched := model.SupplementMatch(row.MatchedBy)
	return row.SellerReport, &matched, nil
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
	rows, err := store.New(s.d.DB.Conn(ctx)).BuyerReports(ctx, params)
	if err != nil {
		return model.ReportPage{}, err
	}
	page := model.ReportPage{Items: make([]model.Report, 0, len(rows))}
	for _, row := range rows {
		page.Items = append(page.Items, toReport(row))
	}
	if len(page.Items) > int(size) {
		page.Items = page.Items[:size]
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
