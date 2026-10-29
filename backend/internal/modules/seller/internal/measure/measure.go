// Package measure 是措施（警告、停權與終止合作，ADR 2026-10-18-seller-governance-consolidated「決策」的「認定、措施與效果」「申訴、補正與凍結」「終止合作」）：
// admin 對帳號做的決定與它的依據（認定、風險類型與契約條款）、修改、結束與撤銷，賣家的申覆與 admin 的裁定，以及內部註記。措施是停權狀態的唯一
// 來源：停權效果（生效與否）在讀取時由措施的狀態、開始時間（starts_at，不可變）與 ends_at（目前值，在 measures）推導（stint.Derive，查詢是
// EffectiveMeasures），停權期間與事件一律由 suspension.Recompute 維護。終止合作撤銷時，它關閉的店家在申覆期間內於同一個 transaction 恢復營業
// （closing.Reopen），以它為來源的關聯終止一併撤銷。狀態改變在寫入的 transaction 內發模組事件，送達由 A8 的 notification 負責。
package measure

import (
	"context"
	"errors"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/closing"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/exposure"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// Identity 是撤銷終止合作需要的 identity API：恢復營業會授予 SELLER，所以要先依 account_id 升序鎖帳號列（鎖順序：帳號列先於店家列）。
type Identity interface {
	LockAccount(context.Context, int64) error
}

// LinkedIdentities 是關聯帳號的身分反查（kyc 持有金鑰環，所以比對在 kyc）：與措施的身分快照相同、屬於 exceptAccountID 以外的帳號；accountID 不為 nil 時只看那個帳號。
type LinkedIdentities interface {
	LinkedAccountHits(ctx context.Context, q *store.Queries, measureID uuid.UUID, exceptAccountID int64, accountID *int64) ([]model.LinkedHit, error)
}

// Deps 外部依賴；clock 由組裝點注入。Closing 是關店與恢復營業（撤銷終止合作時恢復它關閉的店家）。
type Deps struct {
	DB       *db.DB
	Events   *events.Bus
	Jobs     jobs.Enqueuer
	Media    media.Media
	Identity Identity
	Linked   LinkedIdentities
	Closing  *closing.Service
	Exposure *exposure.Service
	Now      func() time.Time
}

// Service 是措施的 use case。
type Service struct {
	d Deps
	// hooks 是測試用的同步點（hooks_integration.go，只在 integration build tag 下有內容，正式程式是空的）。
	hooks requestHooks
}

// New 建立服務。
func New(d Deps) *Service { return &Service{d: d} }

// now 是寫入用的現在：截到微秒（Postgres 的 timestamptz 精度），之後與已存的時間比較才不會被奈秒的差異誤判。
func (s *Service) now() time.Time { return s.d.Now().UTC().Truncate(time.Microsecond) }

func errMeasureNotFound() error { return apperr.NotFound("找不到這筆措施") }

func errMeasureVersion() error {
	return apperr.VersionConflict("措施已被修改，請重新讀取後再送出")
}

func pageSize(size int32) (int32, error) { return pagination.PageSize(size, 20, 100) }

// atMicrosecond 把輸入的時間截到微秒（UTC）。
func atMicrosecond(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	v := t.UTC().Truncate(time.Microsecond)
	return &v
}

// detail 是載入的程度：list 帶依據的認定與請求；admin 讀單筆與寫入的結果另帶請求的佐證與全部版本。
type detail int

const (
	detailList detail = iota
	detailAdmin
)

// hydrate 把一批措施轉成模型並載入關聯（固定幾次查詢，與筆數無關）。
func (s *Service) hydrate(ctx context.Context, q *store.Queries, rows []store.SellerMeasure, level detail, now time.Time) ([]model.Measure, error) {
	out := make([]model.Measure, len(rows))
	if len(rows) == 0 {
		return out, nil
	}
	ids := make([]uuid.UUID, len(rows))
	index := make(map[uuid.UUID]int, len(rows))
	for i, row := range rows {
		out[i] = toMeasure(row, now)
		ids[i] = row.ID
		index[row.ID] = i
	}
	storeIDs := make([]int64, 0, len(rows))
	seen := make(map[int64]bool, len(rows))
	for _, row := range rows {
		if !seen[row.StoreID] {
			seen[row.StoreID] = true
			storeIDs = append(storeIDs, row.StoreID)
		}
	}
	names, err := q.MeasureStoreNames(ctx, storeIDs)
	if err != nil {
		return nil, err
	}
	storeNames := make(map[int64]string, len(names))
	for _, n := range names {
		if n.StoreName != nil {
			storeNames[n.StoreID] = *n.StoreName
		}
	}
	for i, row := range rows {
		out[i].StoreName = storeNames[row.StoreID]
	}
	findings, err := q.MeasureFindings(ctx, ids)
	if err != nil {
		return nil, err
	}
	findingRows := make([]store.SellerViolation, len(findings))
	for i, f := range findings {
		findingRows[i] = f.SellerViolation
	}
	violations, err := toViolations(ctx, q, findingRows)
	if err != nil {
		return nil, err
	}
	for i, f := range findings {
		at := index[f.MeasureID]
		out[at].Findings = append(out[at].Findings, violations[i])
	}
	if err := loadContents(ctx, q, out, index); err != nil {
		return nil, err
	}
	requests, err := q.RequestsByMeasures(ctx, ids)
	if err != nil {
		return nil, err
	}
	evidence := map[uuid.UUID][]model.Evidence{}
	if level == detailAdmin && len(requests) > 0 {
		requestIDs := make([]uuid.UUID, len(requests))
		for i, r := range requests {
			requestIDs[i] = r.ID
		}
		rows, err := q.RequestsEvidence(ctx, requestIDs)
		if err != nil {
			return nil, err
		}
		for _, e := range rows {
			evidence[e.RequestID] = append(evidence[e.RequestID], model.Evidence{AssetID: e.AssetID, ObjectKey: e.ObjectKey})
		}
	}
	for _, r := range requests {
		i := index[r.MeasureID]
		request := toRequest(r)
		if level == detailAdmin {
			request.Evidence = evidence[r.ID]
			if request.Evidence == nil {
				request.Evidence = []model.Evidence{}
			}
		}
		out[i].Requests = append(out[i].Requests, request)
	}
	if err := loadDeadlines(ctx, q, out, now); err != nil {
		return nil, err
	}
	if level == detailAdmin {
		for i, row := range rows {
			revisions, err := q.MeasureRevisions(ctx, row.ID)
			if err != nil {
				return nil, err
			}
			out[i].Revisions = make([]model.MeasureRevision, len(revisions))
			for j, r := range revisions {
				out[i].Revisions[j] = model.MeasureRevision{
					Revision: r.Revision, StatementToSeller: r.StatementToSeller, EndsAt: r.EndsAt, AdminID: r.AdminID, CreatedAt: r.CreatedAt,
				}
				if r.SystemCause != nil {
					cause := model.RevisionSystemCause(*r.SystemCause)
					out[i].Revisions[j].SystemCause = &cause
				}
			}
		}
	}
	return out, nil
}

// loadContents 載入內容下架措施的對象（其他種類沒有，整批沒有內容下架時不查）。
func loadContents(ctx context.Context, q *store.Queries, measures []model.Measure, index map[uuid.UUID]int) error {
	var ids []uuid.UUID
	for _, m := range measures {
		if m.Kind == model.MeasureContentTakedown {
			ids = append(ids, m.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := q.MeasureContents(ctx, ids)
	if err != nil {
		return err
	}
	for _, r := range rows {
		at := index[r.MeasureID]
		measures[at].Contents = append(measures[at].Contents, model.ContentRef{Kind: model.ContentKind(r.ContentKind), ID: r.ContentID})
	}
	return nil
}

// read 讀一筆措施的目前狀態（目前值在 measures、依據與請求）；不存在回 NotFound。
func (s *Service) read(ctx context.Context, q *store.Queries, id uuid.UUID, level detail, now time.Time) (model.Measure, error) {
	row, err := q.MeasureByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Measure{}, errMeasureNotFound()
	}
	if err != nil {
		return model.Measure{}, err
	}
	return s.readRow(ctx, q, row, level, now)
}

func (s *Service) readRow(ctx context.Context, q *store.Queries, row store.SellerMeasure, level detail, now time.Time) (model.Measure, error) {
	measures, err := s.hydrate(ctx, q, []store.SellerMeasure{row}, level, now)
	if err != nil {
		return model.Measure{}, err
	}
	return measures[0], nil
}

func toMeasure(row store.SellerMeasure, now time.Time) model.Measure {
	m := model.Measure{
		ID: row.ID, AccountID: row.AccountID, StoreID: row.StoreID, Kind: model.MeasureKind(row.Kind), Basis: model.Basis(row.Basis),
		ReviewBy: row.ReviewBy, Status: model.MeasureStatus(row.Status), Version: row.Version, CurrentRevision: row.CurrentRevision,
		CreatedByAdminID: row.CreatedByAdminID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		RevokedByAdminID: row.RevokedByAdminID, RevokedAt: row.RevokedAt,
		StatementToSeller: row.StatementToSeller, StartsAt: row.StartsAt, EndsAt: row.EndsAt, SourceMeasureID: row.SourceMeasureID,
	}
	if row.TerminationGround != nil {
		ground := model.TerminationGround(*row.TerminationGround)
		m.TerminationGround = &ground
	}
	if row.RiskGround != nil {
		ground := model.RiskGround(*row.RiskGround)
		m.RiskGround = &ground
	}
	if row.RevokedReason != nil {
		reason := model.RevokeReason(*row.RevokedReason)
		m.RevokedReason = &reason
	}
	m.State = model.StateOf(m.Status, m.Kind, m.StartsAt, m.EndsAt, now)
	m.ReadAt = now
	return m
}

// toViolations 把認定列轉成模型（與輸入同順序），並載入它們的來源通報（依通報 id 排序）。
func toViolations(ctx context.Context, q *store.Queries, rows []store.SellerViolation) ([]model.Violation, error) {
	out := make([]model.Violation, len(rows))
	if len(rows) == 0 {
		return out, nil
	}
	ids := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	links, err := q.FindingReportIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	sources := make(map[uuid.UUID][]uuid.UUID, len(rows))
	for _, l := range links {
		sources[l.ViolationID] = append(sources[l.ViolationID], l.ReportID)
	}
	for i, row := range rows {
		out[i] = toViolation(row, sources[row.ID])
	}
	return out, nil
}

// toViolation 把認定列與它的來源通報轉成模型。
func toViolation(row store.SellerViolation, sourceReportIDs []uuid.UUID) model.Violation {
	v := model.Violation{
		ID: row.ID, StoreID: row.StoreID, AccountID: row.AccountID, SourceReportIDs: sourceReportIDs,
		Category: policy.Category(row.Category), Severity: model.Severity(row.Severity), Status: model.ViolationStatus(row.Status),
		OriginMeasureID: row.OriginMeasureID, CreatedByAdminID: row.CreatedByAdminID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		RevokedByAdminID: row.RevokedByAdminID, RevokedAt: row.RevokedAt,
	}
	if row.EscalatedFrom != nil {
		from := model.Severity(*row.EscalatedFrom)
		v.EscalatedFrom = &from
	}
	if row.RevokedReason != nil {
		reason := model.RevokeReason(*row.RevokedReason)
		v.RevokedReason = &reason
	}
	return v
}

func toRequest(row store.SellerMeasureRequest) model.MeasureRequest {
	r := model.MeasureRequest{
		ID: row.ID, Kind: model.RequestKind(row.Kind), State: model.RequestState(row.State), Reason: row.Reason, Revision: row.Revision,
		SubmittedAt: row.SubmittedAt, DecisionReason: row.DecisionReason, DecidedByAdminID: row.DecidedByAdminID, DecidedAt: row.DecidedAt,
	}
	if row.CloseCause != nil {
		cause := model.RequestCloseCause(*row.CloseCause)
		r.CloseCause = &cause
	}
	return r
}

// Measure 讀取一筆措施（admin）：含依據的認定、請求與佐證、全部版本，以及相同身分的其他帳號（LinkedAccounts）。
func (s *Service) Measure(ctx context.Context, measureID uuid.UUID) (model.Measure, error) {
	var result model.Measure
	err := s.d.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		row, err := q.MeasureByID(ctx, measureID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errMeasureNotFound()
		}
		if err != nil {
			return err
		}
		if result, err = s.readRow(ctx, q, row, detailAdmin, s.now()); err != nil {
			return err
		}
		result.LinkedAccounts, err = s.linkedAccounts(ctx, q, row)
		return err
	})
	return result, err
}

// Current 讀取一筆措施的目前內容（依據與請求，不載入佐證與全部版本），給只需要目前值的呼叫方（例如 update_mask 的合併基準）。
func (s *Service) Current(ctx context.Context, measureID uuid.UUID) (model.Measure, error) {
	var result model.Measure
	err := s.d.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		var err error
		result, err = s.read(ctx, store.New(s.d.DB.Conn(ctx)), measureID, detailList, s.now())
		return err
	})
	return result, err
}

// AccountMeasure 讀取帳號自己的一筆措施（任何一個 stint 上的）；不屬於這個帳號時回 NotFound（不透露別人的措施存在）。
func (s *Service) AccountMeasure(ctx context.Context, accountID int64, measureID uuid.UUID) (model.Measure, error) {
	if err := model.Positive(accountID); err != nil {
		return model.Measure{}, err
	}
	var result model.Measure
	err := s.d.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		row, err := q.MeasureByID(ctx, measureID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && row.AccountID != accountID) {
			return errMeasureNotFound()
		}
		if err != nil {
			return err
		}
		result, err = s.readRow(ctx, q, row, detailList, s.now())
		return err
	})
	return result, err
}

// lockMeasure 先鎖措施所在帳號的全部 stint（stint.LockAccountOf，所有措施的寫入都先鎖它，鎖順序一致），再鎖措施列。
// 店家可以已經關閉：措施的效果以帳號為單位，已關閉 stint 上的措施照樣可以撤銷與申覆。
func (s *Service) lockMeasure(ctx context.Context, q *store.Queries, measureID uuid.UUID) (store.SellerStore, store.SellerMeasure, error) {
	current, err := q.MeasureByID(ctx, measureID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.SellerStore{}, current, errMeasureNotFound()
	}
	if err != nil {
		return store.SellerStore{}, current, err
	}
	locked, err := stint.LockAccountOf(ctx, q, current.StoreID)
	if err != nil {
		return locked, current, err
	}
	current, err = q.LockMeasure(ctx, measureID)
	return locked, current, err
}

// loadDeadlines 一次讀出這批措施的複審期限（ReviewDeadline）與待處理請求的裁定期限（MeasureRequest.DecideDeadline）。成員、開始與期限只在
// seller.review_measure_review、review_measure_appeal、review_measure_lift_request（db/objects.sql）；義務不在時維持 nil。呼叫的是哪支函式
// 只看請求的種類，請求是不是待處理（含解除請求的措施是否仍生效中）由函式判斷，所以不是待處理的請求不在結果裡。
func loadDeadlines(ctx context.Context, q *store.Queries, measures []model.Measure, now time.Time) error {
	var reviewIDs, appealIDs, liftIDs []uuid.UUID
	for _, m := range measures {
		if m.ReviewBy != nil {
			reviewIDs = append(reviewIDs, m.ID)
		}
		for _, r := range m.Requests {
			switch r.Kind {
			case model.RequestKindAppeal:
				appealIDs = append(appealIDs, r.ID)
			case model.RequestKindRemediation:
				liftIDs = append(liftIDs, r.ID)
			}
		}
	}
	review := map[uuid.UUID]*model.Deadline{}
	if len(reviewIDs) > 0 {
		rows, err := q.ReviewMeasureReviewByIDs(ctx, store.ReviewMeasureReviewByIDsParams{At: now, Ids: reviewIDs})
		if err != nil {
			return err
		}
		for _, r := range rows {
			review[r.MeasureID] = &model.Deadline{Start: r.StartAt, Due: r.DueAt}
		}
	}
	request := map[uuid.UUID]*model.Deadline{}
	if len(appealIDs) > 0 {
		rows, err := q.ReviewMeasureAppealByIDs(ctx, store.ReviewMeasureAppealByIDsParams{At: now, Ids: appealIDs})
		if err != nil {
			return err
		}
		for _, r := range rows {
			request[r.RequestID] = &model.Deadline{Start: r.StartAt, Due: r.DueAt}
		}
	}
	if len(liftIDs) > 0 {
		rows, err := q.ReviewMeasureLiftRequestByIDs(ctx, store.ReviewMeasureLiftRequestByIDsParams{At: now, Ids: liftIDs})
		if err != nil {
			return err
		}
		for _, r := range rows {
			request[r.RequestID] = &model.Deadline{Start: r.StartAt, Due: r.DueAt}
		}
	}
	for i := range measures {
		measures[i].ReviewDeadline = review[measures[i].ID]
		for j := range measures[i].Requests {
			measures[i].Requests[j].DecideDeadline = request[measures[i].Requests[j].ID]
		}
	}
	return nil
}
