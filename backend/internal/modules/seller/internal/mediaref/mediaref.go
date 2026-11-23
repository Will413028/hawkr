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

type prefix struct{}

// KindPrefix 是本模組 River job 種類的前綴。
func (prefix) KindPrefix() string { return "seller" }

// SyncArgs 同步一個 ref 的 media 引用（種類 seller.media_sync）。
type SyncArgs = media.SyncArgs[prefix]

// SweepArgs 是保底掃描（種類 seller.media_sweep）。
type SweepArgs = media.SweepArgs[prefix]

// NewSyncer 組出本模組的同步。
func NewSyncer(d *db.DB, m media.Media, jc jobs.Enqueuer, now func() time.Time, logger *slog.Logger) *media.RefSyncer {
	return media.NewRefSyncer[prefix](d, m, jc, now, logger,
		func(c db.DBTX) media.SyncQueries { return media.NewSyncQueries(store.New(c)) },
		func(ctx context.Context, ref media.Ref) ([]string, error) {
			return currentAssets(ctx, store.New(d.Conn(ctx)), ref)
		},
	)
}

// AddWorkers 登記 River worker。
func AddWorkers(workers *river.Workers, s *media.RefSyncer) { media.SyncAddWorkers[prefix](workers, s) }

// PeriodicJobs 回傳啟動即執行、每 15 分鐘的保底掃描。
func PeriodicJobs() []*river.PeriodicJob { return media.SyncPeriodicJobs[prefix]() }

// Changed 在寫入 ref 的業務列的 transaction 內呼叫：把 retired 記成退場、遞增 ref 的版本並排入同步 job。必須在 transaction 內。
func Changed(ctx context.Context, q *store.Queries, jc jobs.Enqueuer, now time.Time, ref media.Ref, retired ...string) error {
	return media.SyncChanged(ctx, media.NewSyncQueries(q), media.SyncEnqueuer[prefix](jc), now, ref, retired...)
}

// Reject 拒絕把已退場的資產寫回 ref；必須在鎖住業務列的 transaction 內呼叫。
func Reject(ctx context.Context, q *store.Queries, ref media.Ref, candidates ...media.SyncCandidate) error {
	return media.SyncReject(ctx, media.NewSyncQueries(q), ref, candidates...)
}
