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
	c := model.Case{ID: r.ID, Target: t, Version: r.Version, OpenedAt: r.OpenedAt, DecidedByAdmin: r.DecidedByAdminID, ClosedAt: r.ClosedAt}
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
	return c
}

// Report 轉換檢舉列；decision 是所屬案件的決定（nil 是進行中），結果由它推導。
func Report(r store.ModerationReport, decision *string) model.Report {
	out := model.Report{
		ID: r.ID, CaseID: r.CaseID, ReporterBuyerID: r.ReporterBuyerID, Category: policy.Category(r.Category),
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

// Cases 列案件（不含檢舉），開案時間新的在前（keyset，不回總數）。篩選的狀態或對象種類不是合法的值回 InvalidInput；size 依 pagination.PageSize。
func (r Reader) Cases(ctx context.Context, f model.CaseFilter, after *model.CaseCursor, size int32) (model.CasePage, error) {
	if f.State != "" && !slices.Contains(model.CaseStates(), f.State) {
		return model.CasePage{}, apperr.InvalidInputOn([]string{"State"}, "%s", text.CaseListStateInvalid())
	}
	if f.TargetKind != "" && !f.TargetKind.Valid() {
		return model.CasePage{}, apperr.InvalidInputOn([]string{"TargetKind"}, "%s", text.CaseListKindInvalid())
	}
	size, err := pagination.PageSize(size, model.DefaultCasePageSize, model.MaxCasePageSize)
	if err != nil {
		return model.CasePage{}, err
	}
	at, id := after.Key()
	rows, err := store.New(r.DB.Conn(ctx)).ListCases(ctx, store.ListCasesParams{
		State: string(f.State), TargetKind: string(f.TargetKind), AfterOpened: at, AfterID: id, MaxRows: size + 1,
	})
	if err != nil {
		return model.CasePage{}, fmt.Errorf("moderation：列出案件：%w", err)
	}
	more := len(rows) > int(size)
	if more {
		rows = rows[:size]
	}
	out := model.CasePage{Cases: make([]model.Case, len(rows))}
	for i, row := range rows {
		out.Cases[i] = Case(row)
	}
	if more {
		last := rows[len(rows)-1]
		out.Next = &model.CaseCursor{OpenedAt: last.OpenedAt, ID: last.ID}
	}
	return out, nil
}
