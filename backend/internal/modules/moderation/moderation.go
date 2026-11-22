// Package moderation 是 moderation 模組的公開 API：社群檢舉的送出、案件與處置。
//
// 職責、不變量與鎖順序見同目錄的 AGENTS.md；行為規格見 backend/docs/modules/moderation.md。
package moderation

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"time"

	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/accountsuspension"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/handle"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/report"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/retention"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/snapshot"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/view"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
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
	// ReverseInput 是撤銷 HIDE 決定的輸入。
	ReverseInput = model.ReverseInput
	// Reversal 是案件上 HIDE 決定的撤銷紀錄。
	Reversal = model.Reversal
	// Snapshot 是案件凍結的內容快照（依對象種類的 tagged union：Social、Buyer、Review、Chat 恰好一個有值）。
	Snapshot = snapshot.Content
	// SocialSnapshot 是貼文、留言與討論區留言的快照。
	SocialSnapshot = snapshot.Social
	// BuyerSnapshot 是被檢舉買家的公開檔案快照。
	BuyerSnapshot = snapshot.Buyer
	// ReviewSnapshot 是評價的快照。
	ReviewSnapshot = snapshot.Review
	// ReviewReplySnapshot 是店家對評價的回覆的快照。
	ReviewReplySnapshot = snapshot.ReviewReply
	// ChatSnapshot 是聊天訊息與前後文的快照。
	ChatSnapshot = snapshot.Chat
	// ChatContextMessage 是聊天前後文的一則。
	ChatContextMessage = snapshot.ContextMessage
	// ReportItem 是檢舉人讀自己檢舉時的一筆。
	ReportItem = model.ReportItem
	// ReportCursor 是檢舉清單的排序鍵。
	ReportCursor = model.ReportCursor
	// ReportPage 是一頁檢舉。
	ReportPage = model.ReportPage
	// CaseState 是案件的狀態篩選。
	CaseState = model.CaseState
	// CaseFilter 是列案件的篩選。
	CaseFilter = model.CaseFilter
	// CaseCursor 是案件清單的排序鍵。
	CaseCursor = model.CaseCursor
	// CasePage 是一頁案件。
	CasePage = model.CasePage
)

// MyReportsMaxPageSize 是檢舉人清單每頁筆數的上限。
const MyReportsMaxPageSize = model.MaxPageSize

const (
	TargetPost          = model.TargetPost
	TargetComment       = model.TargetComment
	TargetForumEntry    = model.TargetForumEntry
	TargetBuyer         = model.TargetBuyer
	TargetReview        = model.TargetReview
	TargetChatMessage   = model.TargetChatMessage
	TargetReviewReply   = model.TargetReviewReply
	DecisionViolation   = model.DecisionViolation
	DecisionNoViolation = model.DecisionNoViolation
	ActionHide          = model.ActionHide
	ActionWarn          = model.ActionWarn
	ActionBan           = model.ActionBan
	ActionTakedown      = model.ActionTakedown
	OutcomeNone         = model.OutcomeNone
	OutcomeViolation    = model.OutcomeViolation
	OutcomeNoViolation  = model.OutcomeNoViolation
	CaseOpen            = model.CaseOpen
	CaseClosed          = model.CaseClosed
	// ChatContextWindow 是聊天對象的證據快照裡，被檢舉訊息前後各保留的訊息數。
	ChatContextWindow = model.ChatContextWindow
	// DailyReportLimit 是每位檢舉人每個台北日最多新增的檢舉數。
	DailyReportLimit = model.DailyReportLimit
	// SnapshotRetention 是結案後保留內容快照的期間。
	SnapshotRetention = model.SnapshotRetention
	// SourceRefPrefix 是寫進各處分紀錄的來源參照的前綴，後面接案件 ID。
	SourceRefPrefix = handle.SourceRefPrefix
)

// Deps 是模組需要的外部依賴，全部必填（Logger 可為空）。
type Deps struct {
	DB         *db.DB
	Social     *social.Module
	Review     *review.Module
	IM         *im.Module
	Seller     *seller.Module
	Buyer      *buyer.Module
	Suspension *accountsuspension.Module
	Media      media.Media
	Jobs       jobs.Enqueuer
	NewID      func() uuid.UUID
	Now        func() time.Time
	Logger     *slog.Logger
}

// Module 是 moderation 模組。
type Module struct {
	d         Deps
	report    *report.Service
	handle    *handle.Service
	retention *Retention
	view      view.Reader
}

// New 建立模組。
func New(d Deps) *Module {
	if d.DB == nil || d.Social == nil || d.Review == nil || d.IM == nil || d.Seller == nil || d.Buyer == nil || d.Suspension == nil || d.Media == nil || d.Jobs == nil || d.NewID == nil || d.Now == nil {
		panic("moderation: missing dependency")
	}
	ret := NewRetention(RetentionDeps{DB: d.DB, Buyer: d.Buyer, Media: d.Media, Jobs: d.Jobs, Now: d.Now, Logger: d.Logger})
	return &Module{
		d:         d,
		report:    report.New(report.Deps{DB: d.DB, Social: d.Social, Review: d.Review, IM: d.IM, Seller: d.Seller, Buyer: d.Buyer, Holds: ret.svc.Holds(), NewID: d.NewID, Now: d.Now}),
		handle:    handle.New(handle.Deps{DB: d.DB, Social: d.Social, Review: d.Review, IM: d.IM, Seller: d.Seller, Buyer: d.Buyer, Suspension: d.Suspension, Audit: adminaudit.New(d.DB, d.Now), Now: d.Now}),
		retention: ret,
		view:      view.Reader{DB: d.DB},
	}
}

// RetentionDeps 是只做保留期與檢舉人資料清除的依賴（不需要 social、seller、accountsuspension）。
type RetentionDeps struct {
	DB     *db.DB
	Buyer  *buyer.Module
	Media  media.Media
	Jobs   jobs.Enqueuer
	Now    func() time.Time
	Logger *slog.Logger
}

// Retention 是 moderation 在背景與事件裡做的事：快照與說明的保留期、快照圖片的 media 引用、檢舉人刪帳的資料清除。worker 與訂閱買家事件的組裝只需要它，
// 不必組出完整的 Module。
type Retention struct{ svc *retention.Service }

// NewRetention 建立 Retention。
func NewRetention(d RetentionDeps) *Retention {
	if d.DB == nil || d.Buyer == nil || d.Media == nil || d.Jobs == nil || d.Now == nil {
		panic("moderation: missing retention dependency")
	}
	return &Retention{svc: retention.New(retention.Deps{DB: d.DB, Buyer: d.Buyer, Media: d.Media, Jobs: d.Jobs, Now: d.Now, Logger: d.Logger})}
}

// PurgeSnapshots 刪除結案超過 SnapshotRetention 的案件的內容快照（釋放它們在 media 的圖片引用）並清掉檢舉說明，回傳刪除的快照數。它是 moderation.purge_snapshots job 的內容，給 worker 與測試呼叫；冪等。
func (r *Retention) PurgeSnapshots(ctx context.Context) (int64, error) {
	return r.svc.PurgeSnapshots(ctx)
}

// SyncSnapshotMedia 同步一個快照在 media 的引用（Link 它現在引用的、Unlink 釋放的資產）一輪；同步 job 做的事，給測試與營運直接呼叫。
func (r *Retention) SyncSnapshotMedia(ctx context.Context, snapshotID uuid.UUID) error {
	_, err := r.svc.Syncer().Sync(ctx, mediaref.SnapshotRef(snapshotID))
	return err
}

// OnBuyerDeletionRequested 是買家進入刪除中時同 transaction 的訂閱者：清掉這位檢舉人全部檢舉的說明與每日計數列，檢舉列與 reporter_buyer_id 留著（A7-D21）。
func (r *Retention) OnBuyerDeletionRequested(ctx context.Context, e buyer.BuyerDeletionRequested) error {
	return r.svc.OnBuyerDeletionRequested(ctx, e)
}

// OnStoreRetired 是店家退場時同 transaction 的訂閱者：清掉該店以檢舉人身分送出全部檢舉的說明、冪等紀錄與每日計數列，檢舉列與 reporter_store_id 留著（A6-D22）。
func (r *Retention) OnStoreRetired(ctx context.Context, e seller.StoreRetired) error {
	return r.svc.OnStoreRetired(ctx, e)
}

// OnBuyerDeleted 是買家關閉時同 transaction 的訂閱者：同 OnBuyerDeletionRequested（冪等）。
func (r *Retention) OnBuyerDeleted(ctx context.Context, e buyer.BuyerDeleted) error {
	return r.svc.OnBuyerDeleted(ctx, e)
}

// AddWorkers 登記 River worker：快照保留期清除與快照圖片引用的同步、保底掃描。
func (r *Retention) AddWorkers(workers *river.Workers) { r.svc.AddWorkers(workers) }

// PeriodicJobs 回傳週期 job：每日清除過期的內容快照與說明、快照圖片引用的保底掃描。
func (r *Retention) PeriodicJobs() []*river.PeriodicJob { return r.svc.PeriodicJobs() }

// SubmitReport 送出一筆檢舉：對象要存在而且公開可讀，不能檢舉自己，每位檢舉人每個台北日最多新增 DailyReportLimit 筆；內容在送出時凍結成快照。
// 這位檢舉人對這個對象在進行中案件裡已有檢舉時回傳既有的那一筆（Created 為假、不吃額度）；案件結案後可以再檢舉（開新案件）。
// 帶 RequestID 時，同一位檢舉人的同一個 RequestID 一律回第一次的結果（含案件已結案之後），內容不同回 InvalidInput。
func (m *Module) SubmitReport(ctx context.Context, in SubmitInput) (SubmitResult, error) {
	return m.report.Submit(ctx, in)
}

// MyReports 列出檢舉人自己的檢舉，新到舊（keyset）：對象、類別、說明與由案件推導的結果（進行中、違規、不違規）；不含案件 ID。
func (m *Module) MyReports(ctx context.Context, buyerID int64, after *ReportCursor, size int32) (ReportPage, error) {
	return m.report.MyReports(ctx, buyerID, after, size)
}

// MyStoreReports 列出店家以檢舉人身分送出的檢舉（只有聊天訊息），形狀、排序與分頁同 MyReports；店家退場後的檢舉列留著但呼叫方不再有「此刻的店」，所以讀不到。
func (m *Module) MyStoreReports(ctx context.Context, storeID int64, after *ReportCursor, size int32) (ReportPage, error) {
	return m.report.MyStoreReports(ctx, storeID, after, size)
}

// DecideCase 處理案件並結案：違規要帶認定類別、處置種類與給作者的理由，處置（social 的隱藏或警告、停權、seller 的內容下架）與結案在同一個 transaction；
// 結案是終態；同 transaction 寫一筆後台動作紀錄（Procedure 是動作名稱）。案件已結案或 Version 過期回 VersionConflict，處置整個回滾。
func (m *Module) DecideCase(ctx context.Context, in DecideInput) (CaseDetail, error) {
	return m.handle.Decide(ctx, in)
}

// ReverseHide 撤銷一個已結案的 HIDE 決定：案件維持結案、決定不改，撤銷（時間、管理員、原因）記在案件上；同 transaction 呼叫擁有內容的模組（review、im、social）
// 還原目前未還原的隱藏，並寫後台動作紀錄（Actor.Action 是動作名稱）。案件不存在回 NotFound；案件進行中、處置不是 HIDE 或已撤銷過回 FailedPrecondition。
func (m *Module) ReverseHide(ctx context.Context, in ReverseInput) (CaseDetail, error) {
	return m.handle.Reverse(ctx, in)
}

// Case 讀一個案件與它的全部檢舉；每筆檢舉的結果由案件的決定推導。案件不存在回 NotFound。
func (m *Module) Case(ctx context.Context, id uuid.UUID) (CaseDetail, error) {
	return m.view.Case(ctx, id)
}

// CaseWithSnapshot 同一個唯讀快照裡讀案件（含檢舉）與最近一份凍結的內容快照（沒有回 nil）；後台 `GetCase` 用，兩者不會來自不同時間點。案件不存在回 NotFound。
func (m *Module) CaseWithSnapshot(ctx context.Context, id uuid.UUID) (CaseDetail, *Snapshot, error) {
	return m.view.CaseWithSnapshot(ctx, id)
}

// Cases 列案件（不含檢舉），開案時間新的在前（keyset）：state 篩進行中或已結案、target kind 篩對象種類，空值是不篩，不合法的值回 InvalidInput。
func (m *Module) Cases(ctx context.Context, f CaseFilter, after *CaseCursor, size int32) (CasePage, error) {
	return m.view.Cases(ctx, f, after, size)
}

// OpenReportCounts 回每篇貼文在進行中案件裡的檢舉數（已結案案件的檢舉不算）；沒有進行中檢舉的貼文不在結果裡，讀的人當 0。後台貼文列表用。
func (m *Module) OpenReportCounts(ctx context.Context, postIDs []uuid.UUID) (map[uuid.UUID]int64, error) {
	return m.view.OpenReportCounts(ctx, postIDs)
}

// Retention 回傳模組的保留期與檢舉人資料清除（serve 以它訂閱買家的刪除事件；給組裝與測試）。
func (m *Module) Retention() *Retention { return m.retention }

// PurgeSnapshots 同 Retention.PurgeSnapshots。
func (m *Module) PurgeSnapshots(ctx context.Context) (int64, error) {
	return m.retention.PurgeSnapshots(ctx)
}

// AddWorkers 登記本模組的 River worker。
func (m *Module) AddWorkers(workers *river.Workers) { m.retention.AddWorkers(workers) }

// PeriodicJobs 回傳週期 job。
func (m *Module) PeriodicJobs() []*river.PeriodicJob { return m.retention.PeriodicJobs() }
