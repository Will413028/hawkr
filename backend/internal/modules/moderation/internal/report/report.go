// Package report 是檢舉的送出：驗證對象存在而且公開可讀、不能檢舉自己、凍結內容快照、開案或掛進進行中的案件、去重與每日上限。
//
// 順序有意義（A7-D14、A7-D20）：先查「這位檢舉人是不是已經檢舉過」（重送回傳既有檢舉、不吃額度，不管內容現在還看不看得到），再做不鎖列的讀取（對象、作者、快照，
// 單體規則 4 由下往上），才扣每日額度（單一 upsert），之後是 moderation 自己的列：開案（ON CONFLICT DO NOTHING 再讀）、掛進案件（版本加一並鎖住案件列）、快照、檢舉。
// 同一位檢舉人的並行重送在最後一步撞到 (case_id, reporter_buyer_id)：整個 transaction 回滾（額度與版本跟著退回）再重來一次，第二次會在「已檢舉過」那一步回傳既有的。
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
	moderationtext "github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/text"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/view"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
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
	Holds  Holds
	NewID  func() uuid.UUID
	Now    func() time.Time
}

// Holds 登記快照引用的圖片資產（A7-D21 證據保全）；必須在呼叫方的 transaction 內。
type Holds interface {
	Hold(ctx context.Context, snapshotID uuid.UUID, now time.Time, assetIDs []string) error
}

// Service 是檢舉送出的 use case。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service { return &Service{d: d} }

// Validate 檢查輸入的形狀並正規化說明（去頭尾空白）；不查資料庫。
func Validate(ctx context.Context, in model.SubmitInput) (model.SubmitInput, error) {
	l := uitext.LocaleFrom(ctx)
	if in.ReporterBuyerID <= 0 {
		return in, apperr.InvalidInputOn([]string{"ReporterBuyerID"}, "%s", moderationtext.ReportReporterRequired(l))
	}
	if !in.Target.Validate() {
		return in, apperr.InvalidInputOn([]string{"Target"}, "%s", moderationtext.ReportTargetInvalid(l))
	}
	if !in.Category.CommunityReporterSelectable() {
		return in, apperr.InvalidInputOn([]string{"Category"}, "%s", moderationtext.ReportCategoryInvalid(l))
	}
	in.Description = trimtext.Trim(in.Description)
	if utf8.RuneCountInString(in.Description) > model.MaxDescriptionRunes {
		return in, apperr.InvalidInputOn([]string{"Description"}, "%s", moderationtext.ReportDescriptionTooLong(l, moderationtext.ReportDescriptionTooLongArgs{Max: model.MaxDescriptionRunes}))
	}
	return in, nil
}

// Submit 送出一筆檢舉。對象不存在、已刪除、被隱藏或不是公開可讀回 NotFound；檢舉自己的內容或自己回 InvalidInput；到每日上限且是新的檢舉回 ResourceExhausted。
// 這位檢舉人對這個對象在進行中案件裡已有檢舉時回傳既有的那一筆（Created 為假），不吃每日額度；案件結案後可以再檢舉（開新案件）。
func (s *Service) Submit(ctx context.Context, in model.SubmitInput) (model.SubmitResult, error) {
	in, err := Validate(ctx, in)
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
	q := store.New(s.d.DB.Conn(ctx))
	existing, err := q.ExistingOpenReport(ctx, store.ExistingOpenReportParams{
		TargetKind: string(in.Target.Kind), ContentID: in.Target.ContentIDPtr(), BuyerID: in.Target.BuyerIDPtr(), ReporterBuyerID: in.ReporterBuyerID,
	})
	switch {
	case err == nil:
		return s.replay(ctx, q, existing)
	case !errors.Is(err, pgx.ErrNoRows):
		return model.SubmitResult{}, fmt.Errorf("moderation：查既有的檢舉：%w", err)
	}

	content, err := s.freeze(ctx, in)
	if err != nil {
		return model.SubmitResult{}, err
	}
	body, sum := snapshot.Encode(content)
	now := s.d.Now().UTC()

	if _, err := q.BumpDailyCount(ctx, store.BumpDailyCountParams{
		ReporterBuyerID: in.ReporterBuyerID, Day: twtime.Day(now), DailyLimit: model.DailyReportLimit,
	}); errors.Is(err, pgx.ErrNoRows) {
		return model.SubmitResult{}, apperr.ResourceExhausted(twtime.Midnight(now).AddDate(0, 0, 1).Sub(now), "%s",
			moderationtext.ReportDailyLimit(uitext.LocaleFrom(ctx), moderationtext.ReportDailyLimitArgs{Max: model.DailyReportLimit}))
	} else if err != nil {
		return model.SubmitResult{}, fmt.Errorf("moderation：計每日檢舉數：%w", err)
	}

	c, created, err := s.openCase(ctx, q, in.Target, now)
	if err != nil {
		return model.SubmitResult{}, err
	}
	attached := c
	if !created {
		// 掛進別人開的案件：版本加一並鎖住案件列，管理員讀到的檢舉集合就不是結案時的集合；自己剛開的案件版本就是 1（開案的那筆檢舉）。
		attached, err = q.AttachToCase(ctx, c.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			return model.SubmitResult{}, errRetry
		}
		if err != nil {
			return model.SubmitResult{}, fmt.Errorf("moderation：掛進案件 %s：%w", c.ID, err)
		}
	}

	snapshotID := s.d.NewID()
	n, err := q.InsertSnapshot(ctx, store.InsertSnapshotParams{
		ID: snapshotID, CaseID: c.ID, ContentSha256: sum[:], Content: body, AssetIds: snapshot.Normalize(content).MediaRefs, CapturedAt: now,
	})
	if err != nil {
		return model.SubmitResult{}, fmt.Errorf("moderation：寫入快照：%w", err)
	}
	if n == 1 {
		if err := s.d.Holds.Hold(ctx, snapshotID, now, snapshot.Normalize(content).MediaRefs); err != nil {
			return model.SubmitResult{}, err
		}
	} else if snapshotID, err = q.SnapshotIDByHash(ctx, store.SnapshotIDByHashParams{CaseID: c.ID, ContentSha256: sum[:]}); err != nil {
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
	return model.SubmitResult{Report: view.Report(row, nil), Case: view.Case(attached), Created: true}, nil
}

// replay 回傳既有的檢舉與它的案件。
func (s *Service) replay(ctx context.Context, q *store.Queries, r store.ModerationReport) (model.SubmitResult, error) {
	row, err := q.GetCase(ctx, r.CaseID)
	if err != nil {
		return model.SubmitResult{}, fmt.Errorf("moderation：讀取案件 %s：%w", r.CaseID, err)
	}
	return model.SubmitResult{Report: view.Report(r, row.Decision), Case: view.Case(row)}, nil
}

// openCase 取得對象進行中的案件，沒有就開一個（created 為真）：INSERT ON CONFLICT DO NOTHING 之後讀進行中的案件（並行開案時等對方提交再讀得到）。
func (s *Service) openCase(ctx context.Context, q *store.Queries, t model.Target, now time.Time) (row store.ModerationCase, created bool, err error) {
	if t.Kind == model.TargetBuyer {
		row, err = q.InsertBuyerCase(ctx, store.InsertBuyerCaseParams{ID: s.d.NewID(), BuyerID: &t.BuyerID, OpenedAt: now})
	} else {
		row, err = q.InsertContentCase(ctx, store.InsertContentCaseParams{ID: s.d.NewID(), TargetKind: string(t.Kind), ContentID: &t.ContentID, OpenedAt: now})
	}
	if err == nil {
		return row, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return store.ModerationCase{}, false, fmt.Errorf("moderation：開案：%w", err)
	}
	row, err = q.OpenCaseByTarget(ctx, store.OpenCaseByTargetParams{TargetKind: string(t.Kind), ContentID: t.ContentIDPtr(), BuyerID: t.BuyerIDPtr()})
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ModerationCase{}, false, errRetry
	}
	if err != nil {
		return store.ModerationCase{}, false, fmt.Errorf("moderation：讀進行中的案件：%w", err)
	}
	return row, false, nil
}

// freeze 驗證對象並取得要凍結的內容：全部是不鎖列的讀取。
func (s *Service) freeze(ctx context.Context, in model.SubmitInput) (snapshot.Content, error) {
	if in.Target.Kind == model.TargetBuyer {
		return s.freezeBuyer(ctx, in)
	}
	l := uitext.LocaleFrom(ctx)
	st := social.Target{Kind: social.TargetKind(in.Target.Kind), ID: in.Target.ContentID}
	snap, err := s.d.Social.ContentSnapshot(ctx, st)
	if err != nil {
		return snapshot.Content{}, err
	}
	if !snap.Exists || !snap.Readable {
		return snapshot.Content{}, apperr.NotFound("%s", moderationtext.ReportContentNotFound(l))
	}
	same, err := s.sameAccount(ctx, in.ReporterBuyerID, snap.Author)
	if err != nil {
		return snapshot.Content{}, err
	}
	if same {
		return snapshot.Content{}, apperr.InvalidInputOn([]string{"Target"}, "%s", moderationtext.ReportSelfContent(l))
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
		return snapshot.Content{}, apperr.NotFound("%s", moderationtext.ReportBuyerNotFound(uitext.LocaleFrom(ctx)))
	}
	same, err := s.sameAccount(ctx, in.ReporterBuyerID, social.Actor{Kind: social.ActorBuyer, ID: in.Target.BuyerID})
	if err != nil {
		return snapshot.Content{}, err
	}
	if same {
		return snapshot.Content{}, apperr.InvalidInputOn([]string{"Target"}, "%s", moderationtext.ReportSelfBuyer(uitext.LocaleFrom(ctx)))
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
