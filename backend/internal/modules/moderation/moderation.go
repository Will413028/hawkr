// Package moderation 是 moderation 模組的公開 API：社群檢舉的送出、案件與處置。
//
// 職責、不變量與鎖順序見同目錄的 AGENTS.md；行為規格見 backend/docs/modules/moderation.md。
package moderation

import (
	"context"
	"embed"
	"io/fs"
	"time"

	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/accountsuspension"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/handle"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/report"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/retention"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/view"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

//go:embed db/migrations/*.sql db/migrations/atlas.sum
var migrationFiles embed.FS

// Migrations 回傳本模組的 migration，給 cmd/hawkr 登記。
func Migrations() fs.FS {
	sub, err := fs.Sub(migrationFiles, "db/migrations")
	if err != nil {
		panic(err) // 只有 embed 的路徑寫錯才會發生，編譯後的 binary 不可能遇到
	}
	return sub
}

type (
	// TargetKind 是檢舉對象的種類。
	TargetKind = model.TargetKind
	// Target 是檢舉對象。
	Target = model.Target
	// Decision 是案件的決定。
	Decision = model.Decision
	// Action 是違規的處置種類。
	Action = model.Action
	// Outcome 是檢舉的結果（由案件推導）。
	Outcome = model.Outcome
	// Case 是一個案件。
	Case = model.Case
	// Report 是一筆檢舉。
	Report = model.Report
	// CaseDetail 是案件加上全部檢舉。
	CaseDetail = model.CaseDetail
	// SubmitInput 是送出檢舉的輸入。
	SubmitInput = model.SubmitInput
	// SubmitResult 是送出檢舉的結果。
	SubmitResult = model.SubmitResult
	// DecideInput 是處理案件的輸入。
	DecideInput = model.DecideInput
)

const (
	TargetPost          = model.TargetPost
	TargetComment       = model.TargetComment
	TargetForumEntry    = model.TargetForumEntry
	TargetBuyer         = model.TargetBuyer
	DecisionViolation   = model.DecisionViolation
	DecisionNoViolation = model.DecisionNoViolation
	ActionHide          = model.ActionHide
	ActionWarn          = model.ActionWarn
	ActionBan           = model.ActionBan
	ActionTakedown      = model.ActionTakedown
	OutcomePending      = model.OutcomePending
	OutcomeViolation    = model.OutcomeViolation
	OutcomeNoViolation  = model.OutcomeNoViolation
	// DailyReportLimit 是每位檢舉人每個台北日最多新增的檢舉數。
	DailyReportLimit = model.DailyReportLimit
	// SnapshotRetention 是結案後保留內容快照的期間。
	SnapshotRetention = model.SnapshotRetention
	// SourceRefPrefix 是寫進各處分紀錄的來源參照的前綴，後面接案件 ID。
	SourceRefPrefix = handle.SourceRefPrefix
)

// Deps 是模組需要的外部依賴，全部必填。
type Deps struct {
	DB         *db.DB
	Social     *social.Module
	Seller     *seller.Module
	Buyer      *buyer.Module
	Suspension *accountsuspension.Module
	NewID      func() uuid.UUID
	Now        func() time.Time
}

// Module 是 moderation 模組。
type Module struct {
	d         Deps
	report    *report.Service
	handle    *handle.Service
	retention *retention.Service
	view      view.Reader
}

// New 建立模組。
func New(d Deps) *Module {
	if d.DB == nil || d.Social == nil || d.Seller == nil || d.Buyer == nil || d.Suspension == nil || d.NewID == nil || d.Now == nil {
		panic("moderation: missing dependency")
	}
	return &Module{
		d:         d,
		report:    report.New(report.Deps{DB: d.DB, Social: d.Social, Seller: d.Seller, Buyer: d.Buyer, NewID: d.NewID, Now: d.Now}),
		handle:    handle.New(handle.Deps{DB: d.DB, Social: d.Social, Seller: d.Seller, Buyer: d.Buyer, Suspension: d.Suspension, Now: d.Now}),
		retention: retention.New(retention.Deps{DB: d.DB, Now: d.Now}),
		view:      view.Reader{DB: d.DB},
	}
}

// SubmitReport 送出一筆檢舉：對象要存在而且公開可讀，不能檢舉自己，每位檢舉人每個台北日最多新增 DailyReportLimit 筆；內容在送出時凍結成快照。
// 這位檢舉人對這個對象在進行中案件裡已有檢舉時回傳既有的那一筆（Created 為假、不吃額度）；案件結案後可以再檢舉（開新案件）。
func (m *Module) SubmitReport(ctx context.Context, in SubmitInput) (SubmitResult, error) {
	return m.report.Submit(ctx, in)
}

// DecideCase 處理案件並結案：違規要帶認定類別、處置種類與給作者的理由，處置（social 的隱藏或警告、停權、seller 的內容下架）與結案在同一個 transaction；
// 結案是終態。案件已結案或 Version 過期回 VersionConflict，處置整個回滾。
func (m *Module) DecideCase(ctx context.Context, in DecideInput) (CaseDetail, error) {
	return m.handle.Decide(ctx, in)
}

// Case 讀一個案件與它的全部檢舉；每筆檢舉的結果由案件的決定推導。案件不存在回 NotFound。
func (m *Module) Case(ctx context.Context, id uuid.UUID) (CaseDetail, error) {
	return m.view.Case(ctx, id)
}

// PurgeSnapshots 刪除結案超過 SnapshotRetention 的案件的內容快照，回傳刪除的快照數。它是 moderation.purge_snapshots job 的內容，給 worker 與測試呼叫；冪等。
func (m *Module) PurgeSnapshots(ctx context.Context) (int64, error) {
	return m.retention.PurgeSnapshots(ctx)
}

// AddWorkers 登記本模組的 River worker。
func (m *Module) AddWorkers(workers *river.Workers) { retention.AddWorkers(workers, m.retention) }

// PeriodicJobs 回傳週期 job：每日清除過期的內容快照。
func (m *Module) PeriodicJobs() []*river.PeriodicJob { return retention.PeriodicJobs() }
