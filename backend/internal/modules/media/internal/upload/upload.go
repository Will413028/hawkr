// Package upload 是暫存上傳的生命週期：BeginUpload、CompleteUpload、Confirm、GetUpload 與處理 job 的狀態轉換。
//
// 狀態機在 db/schema.sql 的 staged_uploads 上方：每個轉換都是條件式 UPDATE（WHERE state=<預期>），受影響 0 列就是輸給對方
// （GC、另一次處理、另一個確認），呼叫端重讀該列判斷原因。呼叫物件儲存的函式（BeginUpload、Process）不收 transaction 參數，
// 並在入口以 model.RequireNoTx 拒絕帶 transaction 的 ctx；Confirm 相反，必須在擁有者的 transaction 內。
package upload

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/storage"
	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// Deps 是服務的依賴。
type Deps struct {
	DB    *db.DB
	Jobs  jobs.Enqueuer
	Store storage.Store
	// Process 是處理 job 的內容：讀暫存物件、驗證、剝除或加密、寫 FinalKey。永久失敗回 model.Permanent，其餘視為暫時失敗。
	Process ProcessFunc
	NewID   func() uuid.UUID
	Now     func() time.Time
	Logger  *slog.Logger
}

// Service 是暫存上傳的服務。
type Service struct{ d Deps }

// New 建立服務；必要的依賴缺少時 panic（組裝錯誤，啟動就該失敗）。
func New(d Deps) *Service {
	if d.DB == nil || d.Jobs == nil || d.Store == nil || d.Process == nil || d.NewID == nil || d.Now == nil {
		panic("media/upload：DB、Jobs、Store、Process、NewID 與 Now 是必要的")
	}
	if model.PresignTTL > model.UploadTTL {
		panic("media/upload：預簽網址的有效期不得超過暫存 TTL")
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &Service{d: d}
}

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

// BeginInput 是 BeginUpload 的宣告；預簽 PUT 的簽章綁這些值。
type BeginInput struct {
	Visibility  model.Visibility
	Size        int64
	ContentType string
	// MD5 是內容 MD5 的標準 base64。
	MD5 string
}

// BeginResult 是 BeginUpload 的結果。Token 只在這裡出現一次，資料表只存雜湊。
type BeginResult struct {
	ID        uuid.UUID
	Token     string
	URL       string
	Headers   map[string]string
	ExpiresAt time.Time
}

// Status 是 GetUpload 的結果。
type Status struct {
	State        model.UploadState
	RejectReason model.RejectReason
	ExpiresAt    time.Time
}

// ConfirmedAsset 是 Confirm 回傳的資產：只描述這筆暫存對應的資產。
type ConfirmedAsset struct {
	ID          uuid.UUID
	Visibility  model.Visibility
	ContentType string
	SizeBytes   int64
}

const (
	tokenBytes        = 32
	maxContentTypeLen = 255
)

func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

func newToken() string {
	var b [tokenBytes]byte
	// crypto/rand.Read 不會回傳錯誤（失敗時直接終止程式）。
	_, _ = rand.Read(b[:])
	return base64.RawURLEncoding.EncodeToString(b[:])
}

// wellFormed 回報 token 的形狀是不是 newToken 產生的；形狀不對的不查資料庫。
func wellFormed(token string) bool {
	return len(token) == base64.RawURLEncoding.EncodedLen(tokenBytes) && !strings.ContainsAny(token, "+/=")
}

func validate(in BeginInput) error {
	if !in.Visibility.Valid() {
		return apperr.InvalidInputOn([]string{"visibility"}, "上傳的可見性不合法")
	}
	if in.Size <= 0 {
		return apperr.InvalidInputOn([]string{"size"}, "上傳的大小必須大於零")
	}
	if in.ContentType == "" || len(in.ContentType) > maxContentTypeLen || strings.ContainsAny(in.ContentType, "\r\n\x00") {
		return apperr.InvalidInputOn([]string{"content_type"}, "上傳的型別不合法")
	}
	sum, err := base64.StdEncoding.DecodeString(in.MD5)
	if err != nil || len(sum) != 16 {
		return apperr.InvalidInputOn([]string{"md5"}, "上傳的 MD5 必須是 16 位元組的 base64")
	}
	return nil
}

func notFound() error { return apperr.NotFound("找不到這筆上傳") }

// own 回傳 token 對應的暫存列；不存在或上傳者不符都回同一個 NotFound，不洩漏 token 是否存在。
func (s *Service) own(ctx context.Context, uploader model.Uploader, token string) (store.MediaStagedUpload, error) {
	if !wellFormed(token) {
		return store.MediaStagedUpload{}, notFound()
	}
	row, err := s.q(ctx).GetUploadByTokenHash(ctx, hashToken(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return store.MediaStagedUpload{}, notFound()
	}
	if err != nil {
		return store.MediaStagedUpload{}, fmt.Errorf("media：讀取暫存上傳：%w", err)
	}
	if row.UploaderID != uploader.ID || row.UploaderAdmin != uploader.Admin {
		return store.MediaStagedUpload{}, notFound()
	}
	return row, nil
}

func prefixOf(v model.Visibility) storage.Prefix {
	if v == model.VisibilitySecure {
		return storage.PrefixSecure
	}
	return storage.PrefixPublic
}

// Begin 建立暫存列並回傳預簽 PUT 與不透明 token。先簽網址、後寫列：寫列失敗時沒有網址外流。不得在 transaction 內呼叫。
func (s *Service) Begin(ctx context.Context, uploader model.Uploader, in BeginInput) (BeginResult, error) {
	if err := model.RequireNoTx(ctx); err != nil {
		return BeginResult{}, err
	}
	if uploader.ID <= 0 {
		return BeginResult{}, apperr.InvalidInputOn([]string{"uploader"}, "上傳者不合法")
	}
	if err := validate(in); err != nil {
		return BeginResult{}, err
	}
	now := s.d.Now()
	id, token, tmpKey := s.d.NewID(), newToken(), storage.NewKey(storage.PrefixTmp)
	presigned, err := s.d.Store.PresignPut(ctx, storage.PresignPutInput{
		Key: tmpKey, ContentLength: in.Size, ContentType: in.ContentType, ContentMD5: in.MD5, Expires: model.PresignTTL,
	})
	if err != nil {
		return BeginResult{}, fmt.Errorf("media：預簽上傳網址：%w", err)
	}
	expiresAt := now.Add(model.UploadTTL)
	err = s.q(ctx).InsertUpload(ctx, store.InsertUploadParams{
		ID: id, TokenHash: hashToken(token), UploaderID: uploader.ID, UploaderAdmin: uploader.Admin,
		Visibility: string(in.Visibility), DeclaredSize: in.Size, DeclaredContentType: in.ContentType, DeclaredMd5: in.MD5,
		TmpKey: tmpKey, ExpiresAt: expiresAt, Now: now,
	})
	if err != nil {
		return BeginResult{}, fmt.Errorf("media：建立暫存上傳：%w", err)
	}
	return BeginResult{ID: id, Token: token, URL: presigned.URL, Headers: presigned.Headers, ExpiresAt: expiresAt}, nil
}

// Complete 驗上傳者，把 PENDING 轉 UPLOADED、記下正式 key，並在同一個 transaction 排處理 job。重入（已經 UPLOADED 或更後面）
// 直接成功，不重複排 job。列已過期回 FailedPrecondition。
func (s *Service) Complete(ctx context.Context, uploader model.Uploader, token string) error {
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		row, err := s.own(ctx, uploader, token)
		if err != nil {
			return err
		}
		now := s.d.Now()
		finalKey := storage.NewKey(prefixOf(model.Visibility(row.Visibility)))
		n, err := s.q(ctx).MarkUploadUploaded(ctx, store.MarkUploadUploadedParams{ID: row.ID, FinalKey: &finalKey, Now: now})
		if err != nil {
			return fmt.Errorf("media：標記已上傳：%w", err)
		}
		if n == 1 {
			return s.enqueueProcess(ctx, row.ID)
		}
		cur, err := s.q(ctx).GetUpload(ctx, row.ID)
		if err != nil {
			return fmt.Errorf("media：重讀暫存上傳：%w", err)
		}
		if st := model.UploadState(cur.State); st == model.StatePending || st == model.StateDeleting || st == model.StateDeleted {
			return apperr.FailedPrecondition("這筆上傳已過期")
		}
		return nil
	})
}

// Get 回傳暫存列的狀態與拒絕原因，給 client 輪詢；只有上傳者本人看得到。
func (s *Service) Get(ctx context.Context, uploader model.Uploader, token string) (Status, error) {
	row, err := s.own(ctx, uploader, token)
	if err != nil {
		return Status{}, err
	}
	st := Status{State: model.UploadState(row.State), ExpiresAt: row.ExpiresAt}
	if row.RejectReason != nil {
		st.RejectReason = model.RejectReason(*row.RejectReason)
	}
	return st, nil
}

// Confirm 在擁有者的 transaction 內確認暫存：只接受 READY、未過期、上傳者相符的列，把暫存列與資產一起轉成確認。重入（已 CONFIRMED）
// 回同一個資產，而且只回這筆暫存列所記的資產。與 GC 同時發生時只有一方成功。ctx 必須帶 transaction，確認才會與業務列一起提交或回滾。
func (s *Service) Confirm(ctx context.Context, uploader model.Uploader, token string) (ConfirmedAsset, error) {
	if err := model.RequireTx(ctx); err != nil {
		return ConfirmedAsset{}, err
	}
	row, err := s.own(ctx, uploader, token)
	if err != nil {
		return ConfirmedAsset{}, err
	}
	assetID, err := s.q(ctx).MarkUploadConfirmed(ctx, store.MarkUploadConfirmedParams{ID: row.ID, Now: s.d.Now()})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return ConfirmedAsset{}, fmt.Errorf("media：確認暫存上傳：%w", err)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		cur, err := s.q(ctx).GetUpload(ctx, row.ID)
		if err != nil {
			return ConfirmedAsset{}, fmt.Errorf("media：重讀暫存上傳：%w", err)
		}
		if model.UploadState(cur.State) != model.StateConfirmed {
			return ConfirmedAsset{}, notConfirmable(cur, s.d.Now())
		}
		return s.assetOf(ctx, cur, true)
	}
	if assetID == nil {
		return ConfirmedAsset{}, fmt.Errorf("media：READY 的暫存上傳 %s 沒有資產", row.ID)
	}
	n, err := s.q(ctx).ActivateAsset(ctx, store.ActivateAssetParams{ID: *assetID, Now: s.d.Now()})
	if err != nil {
		return ConfirmedAsset{}, fmt.Errorf("media：啟用資產：%w", err)
	}
	if n != 1 {
		return ConfirmedAsset{}, fmt.Errorf("media：暫存上傳 %s 的資產 %s 不是 STAGED", row.ID, *assetID)
	}
	row.AssetID = assetID
	return s.assetOf(ctx, row, false)
}

// notConfirmable 把「不能確認」的暫存列翻成呼叫端看得到的錯誤。
func notConfirmable(row store.MediaStagedUpload, now time.Time) error {
	switch model.UploadState(row.State) {
	case model.StateRejected:
		return apperr.FailedPrecondition("這筆上傳被拒絕，不能確認")
	case model.StateDeleting, model.StateDeleted:
		return apperr.FailedPrecondition("這筆上傳已過期")
	case model.StateReady:
		if !row.ExpiresAt.After(now) {
			return apperr.FailedPrecondition("這筆上傳已過期")
		}
		return apperr.FailedPrecondition("這筆上傳正在被處理")
	case model.StatePending, model.StateUploaded, model.StateConfirmed:
		return apperr.FailedPrecondition("這筆上傳還沒處理完成")
	}
	return apperr.FailedPrecondition("這筆上傳還沒處理完成")
}

// assetOf 回傳暫存列所記的資產；requireActive 時資產不是 ACTIVE（例如已被擁有者刪除）回 FailedPrecondition。
func (s *Service) assetOf(ctx context.Context, row store.MediaStagedUpload, requireActive bool) (ConfirmedAsset, error) {
	if row.AssetID == nil {
		return ConfirmedAsset{}, fmt.Errorf("media：暫存上傳 %s 沒有資產", row.ID)
	}
	a, err := s.q(ctx).GetAsset(ctx, *row.AssetID)
	if err != nil {
		return ConfirmedAsset{}, fmt.Errorf("media：讀取資產：%w", err)
	}
	if requireActive && model.AssetState(a.State) != model.AssetActive {
		return ConfirmedAsset{}, apperr.FailedPrecondition("這筆上傳的資產已不可用")
	}
	return ConfirmedAsset{ID: a.ID, Visibility: model.Visibility(a.Visibility), ContentType: a.ContentType, SizeBytes: a.SizeBytes}, nil
}
