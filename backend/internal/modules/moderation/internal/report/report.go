// Package report 是檢舉的送出：驗證對象存在而且公開可讀、不能檢舉自己、凍結內容快照、開案或掛進進行中的案件、去重與每日上限。
//
// 順序有意義（A7-D14、A7-D20）：先做不鎖列的讀取（對象、作者、快照，單體規則 4 由下往上），再查「這位檢舉人是不是已經檢舉過」（重送回傳既有檢舉、不吃額度），
// 才扣每日額度（單一 upsert），之後是 moderation 自己的列：開案（ON CONFLICT DO NOTHING 再讀）、共享鎖住案件、快照、檢舉。同一位檢舉人的並行重送在最後一步
// 撞到 (case_id, reporter_buyer_id)：整個 transaction 回滾（額度跟著退回）再重來一次，第二次會在「已檢舉過」那一步回傳既有的。
package report

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/snapshot"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/view"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

// maxAttempts 是遇到並行競爭（案件在掛上檢舉前被結掉、同一位檢舉人並行重送）時整個 transaction 重來的上限。
const maxAttempts = 4

var errRetry = errors.New("moderation：並行競爭，重來")

// Deps 是送出檢舉的依賴，全部必填。
type Deps struct {
	DB     *db.DB
	Social *social.Module
	Seller *seller.Module
	Buyer  *buyer.Module
	NewID  func() uuid.UUID
	Now    func() time.Time
}

// Service 是檢舉送出的 use case。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service { return &Service{d: d} }

// Validate 檢查輸入的形狀並正規化說明（去頭尾空白）；不查資料庫。
func Validate(in model.SubmitInput) (model.SubmitInput, error) {
	if in.ReporterBuyerID <= 0 {
		return in, apperr.InvalidInputOn([]string{"ReporterBuyerID"}, "檢舉人必填")
	}
	if !in.Target.Validate() {
		return in, apperr.InvalidInputOn([]string{"Target"}, "檢舉對象不合法")
	}
	if !in.Category.CommunityReporterSelectable() {
		return in, apperr.InvalidInputOn([]string{"Category"}, "檢舉原因不合法")
	}
	in.Description = trimtext.Trim(in.Description)
	if utf8.RuneCountInString(in.Description) > model.MaxDescriptionRunes {
		return in, apperr.InvalidInputOn([]string{"Description"}, "檢舉說明不能超過 %d 字", model.MaxDescriptionRunes)
	}
	return in, nil
}

// Submit 送出一筆檢舉。對象不存在、已刪除、被隱藏或不是公開可讀回 NotFound；檢舉自己的內容或自己回 InvalidInput；到每日上限且是新的檢舉回 ResourceExhausted。
// 這位檢舉人對這個對象在進行中案件裡已有檢舉時回傳既有的那一筆（Created 為假），不吃每日額度；案件結案後可以再檢舉（開新案件）。
func (s *Service) Submit(ctx context.Context, in model.SubmitInput) (model.SubmitResult, error) {
	in, err := Validate(in)
	if err != nil {
		return model.SubmitResult{}, err
	}
	for range maxAttempts {
		var out model.SubmitResult
		err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
			var err error
			out, err = s.attempt(ctx, in)
			return err
		})
		if errors.Is(err, errRetry) {
			continue
		}
		return out, err
	}
	return model.SubmitResult{}, fmt.Errorf("moderation：送出檢舉連續 %d 次遇到並行競爭", maxAttempts)
}

func (s *Service) attempt(ctx context.Context, in model.SubmitInput) (model.SubmitResult, error) {
	content, err := s.freeze(ctx, in)
	if err != nil {
		return model.SubmitResult{}, err
	}
	text, sum := snapshot.Encode(content)
	q := store.New(s.d.DB.Conn(ctx))
	now := s.d.Now().UTC()

	existing, err := q.ExistingOpenReport(ctx, store.ExistingOpenReportParams{
		TargetKind: string(in.Target.Kind), TargetKey: in.Target.Key(), ReporterBuyerID: in.ReporterBuyerID,
	})
	switch {
	case err == nil:
		return s.replay(ctx, q, existing)
	case !errors.Is(err, pgx.ErrNoRows):
		return model.SubmitResult{}, fmt.Errorf("moderation：查既有的檢舉：%w", err)
	}

	if _, err := q.BumpDailyCount(ctx, store.BumpDailyCountParams{
		ReporterBuyerID: in.ReporterBuyerID, Day: twtime.Day(now), DailyLimit: model.DailyReportLimit,
	}); errors.Is(err, pgx.ErrNoRows) {
		return model.SubmitResult{}, apperr.ResourceExhausted(twtime.Midnight(now).AddDate(0, 0, 1).Sub(now), "今天的檢舉次數已達 %d 次上限", model.DailyReportLimit)
	} else if err != nil {
		return model.SubmitResult{}, fmt.Errorf("moderation：計每日檢舉數：%w", err)
	}

	c, err := s.openCase(ctx, q, in.Target, now)
	if err != nil {
		return model.SubmitResult{}, err
	}
	locked, err := q.ShareLockCase(ctx, c.ID)
	if err != nil {
		return model.SubmitResult{}, fmt.Errorf("moderation：鎖住案件 %s：%w", c.ID, err)
	}
	if locked.ClosedAt != nil {
		return model.SubmitResult{}, errRetry
	}

	if err := q.InsertSnapshot(ctx, store.InsertSnapshotParams{
		ID: s.d.NewID(), CaseID: c.ID, ContentSha256: sum[:], Content: text, CapturedAt: now,
	}); err != nil {
		return model.SubmitResult{}, fmt.Errorf("moderation：寫入快照：%w", err)
	}
	snapshotID, err := q.SnapshotIDByHash(ctx, store.SnapshotIDByHashParams{CaseID: c.ID, ContentSha256: sum[:]})
	if err != nil {
		return model.SubmitResult{}, fmt.Errorf("moderation：讀取快照：%w", err)
	}
	var desc *string
	if in.Description != "" {
		desc = &in.Description
	}
	row, err := q.InsertReport(ctx, store.InsertReportParams{
		ID: s.d.NewID(), CaseID: c.ID, ReporterBuyerID: in.ReporterBuyerID, Category: string(in.Category),
		Description: desc, SnapshotID: &snapshotID, CreatedAt: now,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.SubmitResult{}, errRetry
	}
	if err != nil {
		return model.SubmitResult{}, fmt.Errorf("moderation：寫入檢舉：%w", err)
	}
	return model.SubmitResult{Report: view.Report(row, nil), Case: view.Case(locked), Created: true}, nil
}

// replay 回傳既有的檢舉與它的案件。
func (s *Service) replay(ctx context.Context, q *store.Queries, r store.ModerationReport) (model.SubmitResult, error) {
	row, err := q.GetCase(ctx, r.CaseID)
	if err != nil {
		return model.SubmitResult{}, fmt.Errorf("moderation：讀取案件 %s：%w", r.CaseID, err)
	}
	return model.SubmitResult{Report: view.Report(r, row.Decision), Case: view.Case(row)}, nil
}

// openCase 取得對象進行中的案件，沒有就開一個：INSERT ON CONFLICT DO NOTHING 之後讀進行中的案件（並行開案時等對方提交再讀得到）。
func (s *Service) openCase(ctx context.Context, q *store.Queries, t model.Target, now time.Time) (store.ModerationCase, error) {
	p := store.InsertCaseParams{ID: s.d.NewID(), TargetKind: string(t.Kind), OpenedAt: now}
	if t.Kind == model.TargetBuyer {
		p.BuyerID = &t.BuyerID
	} else {
		p.ContentID = &t.ContentID
	}
	row, err := q.InsertCase(ctx, p)
	if err == nil {
		return row, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return store.ModerationCase{}, fmt.Errorf("moderation：開案：%w", err)
	}
	row, err = q.OpenCaseByTarget(ctx, store.OpenCaseByTargetParams{TargetKind: string(t.Kind), TargetKey: t.Key()})
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ModerationCase{}, errRetry
	}
	if err != nil {
		return store.ModerationCase{}, fmt.Errorf("moderation：讀進行中的案件：%w", err)
	}
	return row, nil
}

// freeze 驗證對象並取得要凍結的內容：全部是不鎖列的讀取。
func (s *Service) freeze(ctx context.Context, in model.SubmitInput) (snapshot.Content, error) {
	if in.Target.Kind == model.TargetBuyer {
		return s.freezeBuyer(ctx, in)
	}
	st := social.Target{Kind: social.TargetKind(in.Target.Kind), ID: in.Target.ContentID}
	owner, err := s.d.Social.ContentOwner(ctx, st)
	if err != nil {
		return snapshot.Content{}, err
	}
	if !owner.Exists || !owner.Readable {
		return snapshot.Content{}, apperr.NotFound("檢舉的內容不存在")
	}
	same, err := s.sameAccount(ctx, in.ReporterBuyerID, owner.Author)
	if err != nil {
		return snapshot.Content{}, err
	}
	if same {
		return snapshot.Content{}, apperr.InvalidInputOn([]string{"Target"}, "不能檢舉自己的內容")
	}
	snap, err := s.d.Social.ContentSnapshot(ctx, st)
	if err != nil {
		return snapshot.Content{}, err
	}
	if !snap.Exists {
		return snapshot.Content{}, apperr.NotFound("檢舉的內容不存在")
	}
	return snapshot.Content{
		Kind: in.Target.Kind, AuthorKind: string(snap.Author.Kind), AuthorID: snap.Author.ID, StoreID: snap.StoreID,
		Body: snap.Body, MediaRefs: snap.MediaRefs,
	}, nil
}

func (s *Service) freezeBuyer(ctx context.Context, in model.SubmitInput) (snapshot.Content, error) {
	displays, err := s.d.Buyer.BatchDisplay(ctx, []int64{in.Target.BuyerID})
	if err != nil {
		return snapshot.Content{}, err
	}
	if len(displays) != 1 || displays[0].Deleted {
		return snapshot.Content{}, apperr.NotFound("檢舉的買家不存在")
	}
	same, err := s.sameAccount(ctx, in.ReporterBuyerID, social.Actor{Kind: social.ActorBuyer, ID: in.Target.BuyerID})
	if err != nil {
		return snapshot.Content{}, err
	}
	if same {
		return snapshot.Content{}, apperr.InvalidInputOn([]string{"Target"}, "不能檢舉自己")
	}
	c := snapshot.Content{Kind: model.TargetBuyer}
	if displays[0].DisplayName != nil {
		c.DisplayName = *displays[0].DisplayName
	}
	if displays[0].Avatar != nil {
		c.Avatar = *displays[0].Avatar
	}
	return c, nil
}

// sameAccount 說明作者是不是檢舉人自己：買家作者比對帳號（同一個帳號的歷史 stint 也算自己），店家作者比對店家所屬的帳號。
func (s *Service) sameAccount(ctx context.Context, reporterBuyerID int64, author social.Actor) (bool, error) {
	reporter, err := s.d.Buyer.AccountIDByBuyerID(ctx, reporterBuyerID)
	if err != nil {
		return false, err
	}
	switch author.Kind {
	case social.ActorBuyer:
		other, err := s.d.Buyer.AccountIDByBuyerID(ctx, author.ID)
		if apperr.KindOf(err) == apperr.KindNotFound {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return other == reporter, nil
	case social.ActorStore:
		st, err := s.d.Seller.Store(ctx, author.ID)
		if apperr.KindOf(err) == apperr.KindNotFound {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return st.AccountID == reporter, nil
	}
	return false, nil
}
