// Package mediaref 維護 stint 的 logo 與封面、站外業者的 logo 與封面、KYC 證件與治理佐證在 media 的引用。引用只由這個 ref 的
// 同步 job（SyncArgs）寫入：寫入端在 transaction 內寫業務列、把被換掉或終態清掉的資產記成退場並排入同步；同步 job 依業務列
// 目前的狀態 Link，再精確 Unlink 退場的資產。演算法、seq 協定與保底掃描在 media 模組（media.RefSyncer），這裡只提供本模組
// schema 的 SQL、各 ref 目前資產的讀取與 job 的外殼。
package mediaref

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// SyncArgs 同步一個 ref 的 media 引用，以 (ref_type, ref_id) 為唯一鍵：同一個 ref 同時最多一個排隊中或執行中的 job。
type SyncArgs struct {
	RefType string `json:"ref_type" river:"unique"`
	RefID   string `json:"ref_id" river:"unique"`
}

// Kind 是 River 的 job 種類。
func (SyncArgs) Kind() string { return "seller.media_sync" }

// InsertOpts 是同步 job 的排入選項。
func (SyncArgs) InsertOpts() river.InsertOpts { return media.SyncInsertOpts() }

// SweepArgs 是保底掃描：清掉過期的退場紀錄，補排版本比已同步的新的 ref。
type SweepArgs struct{}

// Kind 是 River 的 job 種類。
func (SweepArgs) Kind() string { return "seller.media_sweep" }

// InsertOpts 把同一個週期內的排程與重啟合併。
func (SweepArgs) InsertOpts() river.InsertOpts { return media.SyncSweepInsertOpts() }

// StoreRef 是店家 logo 或封面（refType 是 media.RefTypeSellerLogo 或 media.RefTypeSellerCover）的 ref。
func StoreRef(refType string, storeID int64) media.Ref {
	return media.Ref{Type: refType, ID: strconv.FormatInt(storeID, 10)}
}

// ProspectRef 是站外業者 logo 或封面（refType 是 media.RefTypeSellerProspectLogo 或 RefTypeSellerProspectCover）的 ref。
func ProspectRef(refType string, prospectID int64) media.Ref {
	return media.Ref{Type: refType, ID: strconv.FormatInt(prospectID, 10)}
}

// CertificationRef 是 KYC 申請的證件的 ref。
func CertificationRef(applicationID uuid.UUID) media.Ref {
	return media.Ref{Type: media.RefTypeCertification, ID: applicationID.String()}
}

// EvidenceRef 是通報、措施上請求（申覆）或詢問回覆的佐證的 ref（ref ID 是通報、請求或詢問的 ID，UUIDv7 不會撞）。
func EvidenceRef(refID uuid.UUID) media.Ref {
	return media.Ref{Type: media.RefTypeGovernanceEvidence, ID: refID.String()}
}

// Enqueuer 回傳排入 ref 的同步 job 的函式（ctx 內有 transaction 時在該 transaction 內）。
func Enqueuer(jc jobs.Enqueuer) func(context.Context, media.Ref) error {
	return func(ctx context.Context, ref media.Ref) error {
		opts := SyncArgs{}.InsertOpts()
		return jc.Enqueue(ctx, SyncArgs{RefType: ref.Type, RefID: ref.ID}, &opts)
	}
}

// Changed 在寫入 ref 的業務列的 transaction 內呼叫：把 retired 記成退場、遞增 ref 的版本並排入同步 job。必須在 transaction 內。
func Changed(ctx context.Context, q *store.Queries, jc jobs.Enqueuer, now time.Time, ref media.Ref, retired ...string) error {
	return media.SyncChanged(ctx, queries{q}, Enqueuer(jc), now, ref, retired...)
}

// Reject 拒絕把已退場的資產寫回 ref；必須在鎖住業務列的 transaction 內呼叫。
func Reject(ctx context.Context, q *store.Queries, ref media.Ref, candidates ...media.SyncCandidate) error {
	return media.SyncReject(ctx, queries{q}, ref, candidates...)
}

// NewSyncer 組出本模組的同步。
func NewSyncer(d *db.DB, m media.Media, jc jobs.Enqueuer, now func() time.Time, logger *slog.Logger) *media.RefSyncer {
	return &media.RefSyncer{
		DB: d, Media: m, Now: now, Logger: logger,
		Queries: func(c db.DBTX) media.SyncQueries { return queries{store.New(c)} },
		Current: func(ctx context.Context, ref media.Ref) ([]string, error) {
			return currentAssets(ctx, store.New(d.Conn(ctx)), ref)
		},
		Enqueue: Enqueuer(jc),
	}
}

// currentAssets 是業務列目前指著的資產；沒有列的 ref 是終態，回傳空。
func currentAssets(ctx context.Context, q *store.Queries, ref media.Ref) ([]string, error) {
	switch ref.Type {
	case media.RefTypeSellerLogo, media.RefTypeSellerCover:
		storeID, err := parseID(ref)
		if err != nil {
			return nil, err
		}
		row, err := q.ImageAssets(ctx, storeID)
		if err != nil {
			return nil, ignoreNoRows(err)
		}
		return pointer(map[string]*string{media.RefTypeSellerLogo: row.LogoAssetID, media.RefTypeSellerCover: row.CoverAssetID}[ref.Type]), nil
	case media.RefTypeSellerProspectLogo, media.RefTypeSellerProspectCover:
		prospectID, err := parseID(ref)
		if err != nil {
			return nil, err
		}
		row, err := q.ProspectImageAssets(ctx, prospectID)
		if err != nil {
			return nil, ignoreNoRows(err)
		}
		return pointer(map[string]*string{media.RefTypeSellerProspectLogo: row.LogoAssetID, media.RefTypeSellerProspectCover: row.CoverAssetID}[ref.Type]), nil
	case media.RefTypeCertification:
		id, err := uuid.Parse(ref.ID)
		if err != nil {
			return nil, fmt.Errorf("seller：media ref 的 ID %q 不是申請 ID：%w", ref.ID, err)
		}
		return q.KYCDocumentAssets(ctx, id)
	case media.RefTypeGovernanceEvidence:
		id, err := uuid.Parse(ref.ID)
		if err != nil {
			return nil, fmt.Errorf("seller：media ref 的 ID %q 不是通報、請求或詢問 ID：%w", ref.ID, err)
		}
		return q.EvidenceAssets(ctx, id)
	}
	return nil, fmt.Errorf("seller：不認得的 media ref 類型 %s", ref.Type)
}

func parseID(ref media.Ref) (int64, error) {
	id, err := strconv.ParseInt(ref.ID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("seller：media ref %s 的 ID %q 不是整數：%w", ref.Type, ref.ID, err)
	}
	return id, nil
}

func pointer(assetID *string) []string {
	if assetID == nil {
		return nil
	}
	return []string{*assetID}
}

func ignoreNoRows(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return err
}

type worker struct {
	river.WorkerDefaults[SyncArgs]
	syncer *media.RefSyncer
}

// Work 在 transaction 外呼叫 media；失敗由 River 重試，需要再跑一輪時 snooze，一切就緒時在結算的 transaction 內完成。
func (w *worker) Work(ctx context.Context, job *river.Job[SyncArgs]) error {
	return media.SyncWork(ctx, w.syncer, job, media.Ref{Type: job.Args.RefType, ID: job.Args.RefID})
}

// NextRetry 封頂重試間隔，讓 media 恢復後很快登記（GC 寬限之內）。
func (w *worker) NextRetry(job *river.Job[SyncArgs]) time.Time {
	return media.SyncNextRetry(*job.AttemptedAt, job.Attempt)
}

// Timeout 是一次同步的時間上限。
func (w *worker) Timeout(*river.Job[SyncArgs]) time.Duration { return media.SyncJobTimeout }

type sweepWorker struct {
	river.WorkerDefaults[SweepArgs]
	syncer *media.RefSyncer
}

// Work 執行保底掃描。
func (w *sweepWorker) Work(ctx context.Context, _ *river.Job[SweepArgs]) error {
	_, err := w.syncer.Sweep(ctx)
	return err
}

// Timeout 是一次掃描的時間上限。
func (w *sweepWorker) Timeout(*river.Job[SweepArgs]) time.Duration { return time.Minute }

// AddWorkers 登記 River worker。
func AddWorkers(workers *river.Workers, s *media.RefSyncer) {
	river.AddWorker(workers, &worker{syncer: s})
	river.AddWorker(workers, &sweepWorker{syncer: s})
}

// PeriodicJobs 回傳啟動即執行、每 15 分鐘的保底掃描。
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{river.NewPeriodicJob(
		river.PeriodicInterval(media.SyncSweepInterval),
		func() (river.JobArgs, *river.InsertOpts) { return SweepArgs{}, nil },
		&river.PeriodicJobOpts{ID: "seller.media_sweep", RunOnStart: true},
	)}
}

// queries 把 sqlc 的查詢接成 media.SyncQueries。
type queries struct{ q *store.Queries }

func (a queries) RetireAsset(ctx context.Context, ref media.Ref, assetID string, now time.Time) error {
	return a.q.RetireAsset(ctx, store.RetireAssetParams{RefType: ref.Type, RefID: ref.ID, AssetID: assetID, Now: now})
}

func (a queries) BumpRefSeq(ctx context.Context, ref media.Ref, now time.Time) error {
	return a.q.BumpRefSeq(ctx, store.BumpRefSeqParams{RefType: ref.Type, RefID: ref.ID, Now: now})
}

func (a queries) RefSeq(ctx context.Context, ref media.Ref) (int64, bool, error) {
	seq, err := a.q.RefSeq(ctx, store.RefSeqParams{RefType: ref.Type, RefID: ref.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	return seq, err == nil, err
}

func (a queries) RetiredAssetIDs(ctx context.Context, ref media.Ref) ([]string, error) {
	return a.q.RetiredAssetIDs(ctx, store.RetiredAssetIDsParams{RefType: ref.Type, RefID: ref.ID})
}

func (a queries) RetiredAmong(ctx context.Context, ref media.Ref, assetIDs []string) ([]string, error) {
	return a.q.RetiredAmong(ctx, store.RetiredAmongParams{RefType: ref.Type, RefID: ref.ID, AssetIds: assetIDs})
}

func (a queries) MarkUnlinked(ctx context.Context, ref media.Ref, assetIDs []string, now time.Time) error {
	return a.q.MarkUnlinked(ctx, store.MarkUnlinkedParams{RefType: ref.Type, RefID: ref.ID, AssetIds: assetIDs, Now: &now})
}

func (a queries) SettleRefSeq(ctx context.Context, ref media.Ref, seq int64) (int64, error) {
	return a.q.SettleRefSeq(ctx, store.SettleRefSeqParams{RefType: ref.Type, RefID: ref.ID, Seq: seq})
}

func (a queries) HealingRetired(ctx context.Context, ref media.Ref, healBefore time.Time) (bool, error) {
	return a.q.HealingRetired(ctx, store.HealingRetiredParams{RefType: ref.Type, RefID: ref.ID, HealBefore: &healBefore})
}

func (a queries) PurgeRetired(ctx context.Context, healBefore, retireBefore time.Time) error {
	return a.q.PurgeRetired(ctx, store.PurgeRetiredParams{HealBefore: &healBefore, RetireBefore: retireBefore})
}

func (a queries) UnsyncedRefs(ctx context.Context, idleBefore time.Time, limit int32) ([]media.SyncUnsynced, error) {
	rows, err := a.q.UnsyncedRefs(ctx, store.UnsyncedRefsParams{IdleBefore: idleBefore, MaxRows: limit})
	if err != nil {
		return nil, err
	}
	out := make([]media.SyncUnsynced, len(rows))
	for i, r := range rows {
		out[i] = media.SyncUnsynced{Ref: media.Ref{Type: r.RefType, ID: r.RefID}, UpdatedAt: r.UpdatedAt}
	}
	return out, nil
}

// ReadSecure 讀一份已經記下讀取紀錄的機敏檔（KYC 證件、治理佐證）：呼叫方在 transaction 內寫好讀取紀錄並 commit 之後，在
// transaction 外呼叫這裡。media 讀不到時分成三種：不是機敏資產（media.ErrNotSecure，原樣回錯誤：資料或設定錯）、ref 的引用
// 同步在呼叫之前還沒結算（model.ErrFileNotSynced：新轉正的檔，稍後重試會好）、其餘（model.ErrFileMissing：我們有這份檔而
// media 說沒有，資料不一致，要人處理）。同步狀態在呼叫 media 之前讀，呼叫期間才結算的同步不會被誤判成資料不一致。
func ReadSecure(ctx context.Context, q *store.Queries, m media.Media, ref media.Ref, objectKey string) (model.FileContent, error) {
	settled, err := q.MediaSyncSettled(ctx, store.MediaSyncSettledParams{RefType: ref.Type, RefID: ref.ID})
	if err != nil {
		return model.FileContent{}, err
	}
	asset, err := m.ReadSecure(ctx, objectKey, ref)
	if err == nil {
		return model.FileContent{Data: asset.Data, ContentType: asset.ContentType}, nil
	}
	if errors.Is(err, media.ErrNotSecure) || apperr.KindOf(err) != apperr.KindNotFound {
		return model.FileContent{}, err
	}
	if !settled {
		return model.FileContent{}, model.ErrFileNotSynced
	}
	return model.FileContent{}, model.ErrFileMissing
}

// EvidenceTokens 檢查佐證的 staged token：最多 model.MaxEvidence 張，不可為空白。
func EvidenceTokens(tokens []string) error {
	if len(tokens) > model.MaxEvidence {
		return apperr.InvalidInputOn([]string{"EvidenceTokens"}, "佐證最多 %d 張", model.MaxEvidence)
	}
	for i, token := range tokens {
		if strings.TrimSpace(token) == "" {
			return apperr.InvalidInputOn([]string{apperr.IndexedField("EvidenceTokens", i)}, "佐證不可為空白")
		}
	}
	return nil
}

// FinalizeEvidence 在 transaction 外、寫入業務列之前把佐證轉正（不建立引用）；引用由寫入的 transaction 排入的同步 job 登記
// （Changed），轉正之後沒有寫入的資產沒有引用，由 media 的 GC 在建立滿 7 天後回收。
func FinalizeEvidence(ctx context.Context, m media.Media, uploader media.Uploader, tokens []string) ([]model.Evidence, error) {
	trimmed := make([]string, len(tokens))
	for i, token := range tokens {
		trimmed[i] = strings.TrimSpace(token)
	}
	// 逐張轉正並檢查，不合格的一次全部回報：media 拒絕的 token（無效、過期或不屬於這個帳號）與不是機敏檔的佐證（佐證必須是機敏檔，
	// 只經後台的擁有者端點讀、每次讀取都記錄）各指到那一張（EvidenceTokens[i]，整個欄位的問題如張數上限指 EvidenceTokens）。
	// 失敗的請求已轉正的資產沒有引用，由 media 的 GC 在建立滿 7 天後回收。
	assets, err := media.FinalizeList(ctx, m, uploader, "EvidenceTokens", trimmed, media.RefTypeGovernanceEvidence, func(_ int, asset media.Asset) string {
		if !asset.Secure {
			return "佐證必須以機敏上傳，請重新上傳"
		}
		return ""
	})
	if err != nil {
		return nil, err
	}
	evidence := make([]model.Evidence, 0, len(assets))
	for _, asset := range assets {
		evidence = append(evidence, model.Evidence{AssetID: asset.ID, ObjectKey: asset.ObjectKey})
	}
	return evidence, nil
}
