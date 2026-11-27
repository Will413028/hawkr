// Package view 把 store 的列轉成對外型別，並讀案件加檢舉（結果由案件的決定推導）。
package view

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/snapshot"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// Case 轉換案件列。
func Case(r store.ModerationCase) model.Case {
	t := model.Target{Kind: model.TargetKind(r.TargetKind)}
	if r.ContentID != nil {
		t.ContentID = *r.ContentID
	}
	if r.BuyerID != nil {
		t.BuyerID = *r.BuyerID
	}
	c := model.Case{ID: r.ID, Target: t, Version: r.Version, OpenedAt: r.OpenedAt, DecidedByAdmin: r.DecidedByAdminID, ClosedAt: r.ClosedAt, MeasureID: r.MeasureID}
	if r.Decision != nil {
		d := model.Decision(*r.Decision)
		c.Decision = &d
	}
	if r.DecisionCategory != nil {
		p := policy.Category(*r.DecisionCategory)
		c.Category = &p
	}
	if r.DecisionAction != nil {
		a := model.Action(*r.DecisionAction)
		c.Action = &a
	}
	if r.ReversedAt != nil && r.ReversedByAdminID != nil && r.ReverseReason != nil {
		c.Reversal = &model.Reversal{At: *r.ReversedAt, AdminID: *r.ReversedByAdminID, Reason: *r.ReverseReason}
	}
	return c
}

// Report 轉換檢舉列；decision 是所屬案件的決定（nil 是進行中），結果由它推導。
func Report(r store.ModerationReport, decision *string) model.Report {
	out := model.Report{
		ID: r.ID, CaseID: r.CaseID, ReporterBuyerID: deref(r.ReporterBuyerID), ReporterStoreID: deref(r.ReporterStoreID), Category: policy.Category(r.Category),
		SnapshotID: r.SnapshotID, CreatedAt: r.CreatedAt, Outcome: model.OutcomeOf(decision),
	}
	if r.Description != nil {
		out.Description = *r.Description
	}
	return out
}

// Detail 讀一個案件與它的全部檢舉；案件不存在回 pgx.ErrNoRows。
func Detail(ctx context.Context, q *store.Queries, id uuid.UUID) (model.CaseDetail, error) {
	row, err := q.GetCase(ctx, id)
	if err != nil {
		return model.CaseDetail{}, err
	}
	reports, err := q.ReportsOfCase(ctx, id)
	if err != nil {
		return model.CaseDetail{}, err
	}
	out := model.CaseDetail{Case: Case(row), Reports: make([]model.Report, 0, len(reports))}
	out.Case.ReportCount = int64(len(reports))
	if body, err := q.LatestSnapshotContent(ctx, id); err == nil {
		out.Case.StoreID = snapshot.StoreOf(body)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return model.CaseDetail{}, err
	}
	for _, r := range reports {
		out.Reports = append(out.Reports, Report(r, row.Decision))
	}
	return out, nil
}

// Reader 是案件的讀取入口。
type Reader struct{ DB *db.DB }

// Case 讀一個案件與它的全部檢舉；案件不存在回 NotFound。
func (r Reader) Case(ctx context.Context, id uuid.UUID) (model.CaseDetail, error) {
	d, err := Detail(ctx, store.New(r.DB.Conn(ctx)), id)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.CaseDetail{}, apperr.NotFound("%s", text.CaseNotFound())
	}
	return d, err
}

// Snapshot 讀案件最近一份凍結的內容快照；案件不存在回 NotFound，沒有快照（已過保留期被清除）回 nil。
func (r Reader) Snapshot(ctx context.Context, id uuid.UUID) (*snapshot.Content, error) {
	q := store.New(r.DB.Conn(ctx))
	if _, err := q.GetCase(ctx, id); errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("%s", text.CaseNotFound())
	} else if err != nil {
		return nil, fmt.Errorf("moderation：讀取案件 %s：%w", id, err)
	}
	body, err := q.LatestSnapshotContent(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("moderation：讀取案件 %s 的快照：%w", id, err)
	}
	c, err := snapshot.Decode(body)
	if err != nil {
		return nil, fmt.Errorf("moderation：解讀案件 %s 的快照：%w", id, err)
	}
	return &c, nil
}

// CaseWithSnapshot 在同一個唯讀快照（InReadSnapshot）裡讀案件與它最近的快照，兩者來自同一個時間點（不會一個是結案前、一個是清除後）；
// 案件不存在回 NotFound，沒有快照回 nil。ctx 不可已在 transaction 內。
func (r Reader) CaseWithSnapshot(ctx context.Context, id uuid.UUID) (model.CaseDetail, *snapshot.Content, error) {
	var (
		d    model.CaseDetail
		snap *snapshot.Content
	)
	err := r.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		var err error
		if d, err = r.Case(ctx, id); err != nil {
			return err
		}
		snap, err = r.Snapshot(ctx, id)
		return err
	})
	return d, snap, err
}

// Cases 列案件（不含檢舉），開案時間新的在前（keyset，不回總數）。篩選的狀態或對象種類不是合法的值回 InvalidInput；size 依 pagination.PageSize。
func (r Reader) Cases(ctx context.Context, f model.CaseFilter, after *model.CaseCursor, size int32) (model.CasePage, error) {
	if f.State != "" && !slices.Contains(model.CaseStates(), f.State) {
		return model.CasePage{}, apperr.InvalidInputOn([]string{"State"}, "%s", text.CaseListStateInvalid())
	}
	if f.TargetKind != "" && !f.TargetKind.Valid() {
		return model.CasePage{}, apperr.InvalidInputOn([]string{"TargetKind"}, "%s", text.CaseListKindInvalid())
	}
	size, err := pagination.PageSize(size, model.DefaultPageSize, model.MaxPageSize)
	if err != nil {
		return model.CasePage{}, err
	}
	at, id := after.Key()
	q := store.New(r.DB.Conn(ctx))
	p := store.ListAllCasesParams{TargetKind: string(f.TargetKind), AfterOpened: at, AfterID: id, MaxRows: size + 1}
	var rows []store.ModerationCase
	switch f.State {
	case model.CaseOpen:
		rows, err = q.ListOpenCases(ctx, store.ListOpenCasesParams(p))
	case model.CaseClosed:
		rows, err = q.ListClosedCases(ctx, store.ListClosedCasesParams(p))
	default:
		rows, err = q.ListAllCases(ctx, p)
	}
	if err != nil {
		return model.CasePage{}, fmt.Errorf("moderation：列出案件：%w", err)
	}
	rows, more := model.TrimPage(rows, size)
	out := model.CasePage{Cases: make([]model.Case, len(rows))}
	for i, row := range rows {
		out.Cases[i] = Case(row)
	}
	if err := r.annotate(ctx, q, rows, out.Cases); err != nil {
		return model.CasePage{}, err
	}
	if more {
		last := rows[len(rows)-1]
		out.Next = &model.CaseCursor{OpenedAt: last.OpenedAt, ID: last.ID}
	}
	return out, nil
}

func (r Reader) annotate(ctx context.Context, q *store.Queries, rows []store.ModerationCase, cases []model.Case) error {
	if len(rows) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	counts, err := q.ReportCountsOfCases(ctx, ids)
	if err != nil {
		return fmt.Errorf("moderation：計算案件的檢舉數：%w", err)
	}
	snaps, err := q.LatestSnapshotContentsOfCases(ctx, ids)
	if err != nil {
		return fmt.Errorf("moderation：讀取案件的快照：%w", err)
	}
	reports := make(map[uuid.UUID]int64, len(counts))
	for _, c := range counts {
		reports[c.CaseID] = c.Reports
	}
	stores := make(map[uuid.UUID]*int64, len(snaps))
	for _, s := range snaps {
		stores[s.CaseID] = snapshot.StoreOf(s.Content)
	}
	for i := range cases {
		cases[i].ReportCount = reports[cases[i].ID]
		cases[i].StoreID = stores[cases[i].ID]
	}
	return nil
}

// OpenReportCounts 回每篇貼文在進行中案件裡的檢舉數；只算進行中案件，已結案案件的檢舉不算，沒有進行中檢舉的貼文不在結果裡（讀的人當 0）。
func (r Reader) OpenReportCounts(ctx context.Context, postIDs []uuid.UUID) (map[uuid.UUID]int64, error) {
	out := make(map[uuid.UUID]int64, len(postIDs))
	if len(postIDs) == 0 {
		return out, nil
	}
	rows, err := store.New(r.DB.Conn(ctx)).OpenReportCountsOfPosts(ctx, postIDs)
	if err != nil {
		return nil, fmt.Errorf("moderation：計算貼文的進行中檢舉數：%w", err)
	}
	for _, row := range rows {
		out[*row.PostID] = row.Reports
	}
	return out, nil
}

func deref(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// CaseReporters 是案件一邊的檢舉人（買家或店家），依檢舉人號碼由小到大的 keyset，從 afterID 之後（不含）讀，給結案通知分頁 fan-out（worker 只組 Reader，
// 不需要完整的 Module）；案件沒有這邊的檢舉人或不存在都回空頁。
func (r Reader) CaseReporters(ctx context.Context, caseID uuid.UUID, side model.ReporterSide, afterID int64, size int32) (model.ReporterPage, error) {
	size, err := pagination.PageSize(size, model.DefaultPageSize, model.MaxPageSize)
	if err != nil {
		return model.ReporterPage{}, err
	}
	q := store.New(r.DB.Conn(ctx))
	var rows []store.BuyerReportersOfCaseRow
	switch side {
	case model.ReporterSideBuyer:
		rows, err = q.BuyerReportersOfCase(ctx, store.BuyerReportersOfCaseParams{CaseID: caseID, AfterID: afterID, MaxRows: size + 1})
	case model.ReporterSideStore:
		var stores []store.StoreReportersOfCaseRow
		stores, err = q.StoreReportersOfCase(ctx, store.StoreReportersOfCaseParams{CaseID: caseID, AfterID: afterID, MaxRows: size + 1})
		for _, s := range stores {
			rows = append(rows, store.BuyerReportersOfCaseRow(s))
		}
	default:
		return model.ReporterPage{}, fmt.Errorf("moderation：檢舉人的一邊 %q 不是 BUYER 或 STORE", side)
	}
	if err != nil {
		return model.ReporterPage{}, fmt.Errorf("moderation：列出案件 %s 的檢舉人：%w", caseID, err)
	}
	rows, more := model.TrimPage(rows, size)
	out := model.ReporterPage{Items: make([]model.Reporter, len(rows))}
	for i, row := range rows {
		out.Items[i] = model.Reporter{ReportID: row.ID}
		if side == model.ReporterSideBuyer {
			out.Items[i].BuyerID = row.ReporterID
		} else {
			out.Items[i].StoreID = row.ReporterID
		}
	}
	if more {
		out.Next = rows[len(rows)-1].ReporterID
	}
	return out, nil
}
