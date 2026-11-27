// Package report 是檢舉的送出：驗證對象存在而且公開可讀、不能檢舉自己、凍結內容快照、開案或掛進進行中的案件、去重與每日上限。
//
// 順序有意義（A7-D14、A7-D20）：先查「這位檢舉人是不是已經檢舉過」（重送回傳既有檢舉、不吃額度，不管內容現在還看不看得到），再做不鎖列的讀取（對象、作者、快照，
// 單體規則 4 由下往上），才扣每日額度（單一 upsert），之後是 moderation 自己的列：開案（ON CONFLICT DO NOTHING 再讀）、掛進案件（版本加一並鎖住案件列）、快照、檢舉。
// 同一位檢舉人的並行重送在最後一步撞到 (case_id, reporter_buyer_id)：整個 transaction 回滾（額度與版本跟著退回）再重來一次，第二次會在「已檢舉過」那一步回傳既有的。
package report

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/snapshot"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/store"
	moderationtext "github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/text"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/view"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/idempotency"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

const maxAttempts = 4

const maxCopyRounds = 3

var errRetry = errors.New("moderation：並行競爭，重來")

// Reviews 是送出檢舉用到的 review 方法（*review.Module）：唯讀的證據讀取。
type Reviews interface {
	ReviewForReport(ctx context.Context, id uuid.UUID, viewerBuyerID int64) (review.ReportedReview, error)
}

// Chats 是送出檢舉用到的 im 方法（*im.Module）：唯讀的證據讀取。
type Chats interface {
	MessageForReport(ctx context.Context, messageID uuid.UUID, reporter im.Party, window int) (im.ReportedMessage, error)
}

// Deps 是送出檢舉的依賴，全部必填。
type Deps struct {
	DB     *db.DB
	Events *events.Bus
	Social *social.Module
	Review Reviews
	IM     Chats
	Seller *seller.Module
	Buyer  *buyer.Module
	Media  Media
	NewID  func() uuid.UUID
	Now    func() time.Time
}

// Media 是證據快照對 media 的依賴（B1-D20）：圖片在 transaction 外複製成新資產，在建快照的 transaction 內 Confirm；擁有者的流程沒有 MarkDeleted，
// 快照的複本只由保留期清理在快照過期時刪除（retention）。
type Media interface {
	CopyPublic(ctx context.Context, uploader media.Uploader, sourceID uuid.UUID) (media.PutPublicResult, error)
	Confirm(ctx context.Context, uploader media.Uploader, token string, want media.Visibility) (media.ConfirmedAsset, error)
}

type needCopies struct{ sources []string }

func (needCopies) Error() string { return "moderation：快照的圖片還沒有複本" }

// Service 是檢舉送出的 use case。
type Service struct {
	d         Deps
	accountOf func(ctx context.Context, in model.SubmitInput) (int64, error)
}

// New 建立服務。
func New(d Deps) *Service {
	s := &Service{d: d}
	s.accountOf = s.reporterAccountID
	return s
}

// Validate 檢查輸入的形狀並正規化說明（去頭尾空白）；不查資料庫。
func Validate(ctx context.Context, in model.SubmitInput) (model.SubmitInput, error) {
	l := uitext.LocaleFrom(ctx)
	if (in.ReporterBuyerID > 0) == (in.ReporterStoreID > 0) || in.ReporterBuyerID < 0 || in.ReporterStoreID < 0 {
		fields := []string{"ReporterBuyerID", "ReporterStoreID"}
		switch {
		case in.ReporterBuyerID < 0 && in.ReporterStoreID <= 0:
			fields = fields[:1]
		case in.ReporterStoreID < 0 && in.ReporterBuyerID <= 0:
			fields = fields[1:]
		}
		return in, apperr.InvalidInputOn(fields, "%s", moderationtext.ReportReporterRequired(l))
	}
	if !in.Target.Validate() || (in.ReporterStoreID > 0 && in.Target.Kind != model.TargetChatMessage) {
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
	copies := map[string]string{}
	sourceGone := false
	for attempts, rounds := 0, 0; attempts < maxAttempts; {
		var out model.SubmitResult
		err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
			var err error
			out, err = s.attempt(ctx, in, copies)
			return err
		})
		var need needCopies
		switch {
		case errors.As(err, &need):
			if rounds++; rounds > maxCopyRounds {
				return model.SubmitResult{}, copyRoundsExhausted(ctx, sourceGone)
			}
			if err := s.copySources(ctx, in, need.sources, copies); err != nil {
				if apperr.KindOf(err) != apperr.KindNotFound {
					return model.SubmitResult{}, err
				}
				sourceGone = true
				attempts++
			}
		case errors.Is(err, errRetry):
			attempts++
		default:
			return out, err
		}
	}
	return model.SubmitResult{}, fmt.Errorf("moderation：送出檢舉連續 %d 次遇到並行競爭", maxAttempts)
}

func copyRoundsExhausted(ctx context.Context, sourceGone bool) error {
	if sourceGone {
		return apperr.NotFound("%s", moderationtext.ReportContentNotFound(uitext.LocaleFrom(ctx)))
	}
	return fmt.Errorf("moderation：送出檢舉連續 %d 輪複製證據圖片，內容仍在變動", maxCopyRounds)
}

func (s *Service) uploader(ctx context.Context, in model.SubmitInput) (media.Uploader, error) {
	accountID, err := s.accountOf(ctx, in)
	if err != nil {
		return media.Uploader{}, err
	}
	return media.Uploader{ID: accountID}, nil
}

func (s *Service) reporterAccountID(ctx context.Context, in model.SubmitInput) (int64, error) {
	if in.ReporterStoreID > 0 {
		st, err := s.d.Seller.Store(ctx, in.ReporterStoreID)
		if err != nil {
			return 0, err
		}
		return st.AccountID, nil
	}
	return s.d.Buyer.AccountIDByBuyerID(ctx, in.ReporterBuyerID)
}

func (s *Service) copySources(ctx context.Context, in model.SubmitInput, sources []string, copies map[string]string) error {
	up, err := s.uploader(ctx, in)
	if err != nil {
		return err
	}
	for _, src := range sources {
		if _, ok := copies[src]; ok {
			continue
		}
		id, err := uuid.Parse(src)
		if err != nil {
			return fmt.Errorf("moderation：快照的圖片資產 ID %q 不是 UUID：%w", src, err)
		}
		res, err := s.d.Media.CopyPublic(ctx, up, id)
		if err != nil {
			return err
		}
		copies[src] = res.Token
	}
	return nil
}

func (s *Service) ownCopies(ctx context.Context, in model.SubmitInput, content snapshot.Content, sources []string, copies map[string]string) (snapshot.Content, []uuid.UUID, error) {
	var missing []string
	for _, src := range sources {
		if _, ok := copies[src]; !ok {
			missing = append(missing, src)
		}
	}
	if len(missing) > 0 {
		return snapshot.Content{}, nil, needCopies{sources: missing}
	}
	up, err := s.uploader(ctx, in)
	if err != nil {
		return snapshot.Content{}, nil, err
	}
	to := make(map[string]string, len(sources))
	owned := make([]uuid.UUID, 0, len(sources))
	for _, src := range sources {
		a, err := s.d.Media.Confirm(ctx, up, copies[src], media.VisibilityPublic)
		if err != nil {
			return snapshot.Content{}, nil, fmt.Errorf("moderation：確認證據圖片 %s 的複本：%w", src, err)
		}
		to[src] = a.ID.String()
		owned = append(owned, a.ID)
	}
	slices.SortFunc(owned, func(a, b uuid.UUID) int { return bytes.Compare(a[:], b[:]) })
	return content.ReplaceAssets(to), owned, nil
}

func (s *Service) lockReporter(ctx context.Context, in model.SubmitInput) error {
	var err error
	if in.ReporterStoreID > 0 {
		err = s.d.Seller.LockNotRetired(ctx, in.ReporterStoreID)
	} else {
		err = s.d.Buyer.LockNotClosed(ctx, in.ReporterBuyerID)
	}
	if apperr.KindOf(err) == apperr.KindNotFound {
		return apperr.NotFound("%s", moderationtext.ReportReporterNotFound(uitext.LocaleFrom(ctx)))
	}
	return err
}

func (s *Service) attempt(ctx context.Context, in model.SubmitInput, copies map[string]string) (model.SubmitResult, error) {
	if err := s.lockReporter(ctx, in); err != nil {
		return model.SubmitResult{}, err
	}
	q := store.New(s.d.DB.Conn(ctx))
	var hash []byte
	if in.RequestID != nil {
		hash = requestHash(in)
		first, err := q.ReportByRequest(ctx, store.ReportByRequestParams{ReporterBuyerID: buyerPtr(in), ReporterStoreID: storePtr(in), RequestID: *in.RequestID})
		switch {
		case err == nil:
			if !bytes.Equal(first.RequestHash, hash) {
				return model.SubmitResult{}, apperr.InvalidInputOn([]string{"RequestID"}, "%s", moderationtext.ReportRequestIdReused(uitext.LocaleFrom(ctx)))
			}
			return s.replay(ctx, q, first.ModerationReport)
		case !errors.Is(err, pgx.ErrNoRows):
			return model.SubmitResult{}, fmt.Errorf("moderation：查 request_id 的檢舉：%w", err)
		}
	}
	existing, err := q.ExistingOpenReport(ctx, store.ExistingOpenReportParams{
		TargetKind: string(in.Target.Kind), ContentID: in.Target.ContentIDPtr(), BuyerID: in.Target.BuyerIDPtr(), ReporterBuyerID: buyerPtr(in), ReporterStoreID: storePtr(in),
	})
	switch {
	case err == nil:
		if err := s.recordRequest(ctx, q, in, existing.ID, hash); err != nil {
			return model.SubmitResult{}, err
		}
		return s.replay(ctx, q, existing)
	case !errors.Is(err, pgx.ErrNoRows):
		return model.SubmitResult{}, fmt.Errorf("moderation：查既有的檢舉：%w", err)
	}

	content, err := s.freeze(ctx, in)
	if err != nil {
		return model.SubmitResult{}, err
	}
	_, sum := snapshot.Encode(content)
	now := s.d.Now().UTC()

	if _, err := s.bump(ctx, q, in, now); errors.Is(err, pgx.ErrNoRows) {
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
		attached, err = q.AttachToCase(ctx, c.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			return model.SubmitResult{}, errRetry
		}
		if err != nil {
			return model.SubmitResult{}, fmt.Errorf("moderation：掛進案件 %s：%w", c.ID, err)
		}
	}

	stored, owned := content, []uuid.UUID{}
	if sources := content.AssetIDs(); len(sources) > 0 {
		_, err := q.SnapshotIDByHash(ctx, store.SnapshotIDByHashParams{CaseID: c.ID, ContentSha256: sum[:]})
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			if stored, owned, err = s.ownCopies(ctx, in, content, sources, copies); err != nil {
				return model.SubmitResult{}, err
			}
		case err != nil:
			return model.SubmitResult{}, fmt.Errorf("moderation：查既有的快照：%w", err)
		}
	}
	body, _ := snapshot.Encode(stored)

	snapshotID := s.d.NewID()
	n, err := q.InsertSnapshot(ctx, store.InsertSnapshotParams{
		ID: snapshotID, CaseID: c.ID, ContentSha256: sum[:], Content: body, AssetIds: owned, CapturedAt: now,
	})
	if err != nil {
		return model.SubmitResult{}, fmt.Errorf("moderation：寫入快照：%w", err)
	}
	if n == 0 && len(owned) > 0 {
		return model.SubmitResult{}, errRetry
	}
	if n == 0 {
		if snapshotID, err = q.SnapshotIDByHash(ctx, store.SnapshotIDByHashParams{CaseID: c.ID, ContentSha256: sum[:]}); err != nil {
			return model.SubmitResult{}, fmt.Errorf("moderation：讀取快照：%w", err)
		}
	}
	var desc *string
	if in.Description != "" {
		desc = &in.Description
	}
	row, err := q.InsertReport(ctx, store.InsertReportParams{
		ID: s.d.NewID(), CaseID: c.ID, ReporterBuyerID: buyerPtr(in), ReporterStoreID: storePtr(in), Category: string(in.Category),
		Description: desc, SnapshotID: &snapshotID, CreatedAt: now,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.SubmitResult{}, errRetry
	}
	if err != nil {
		return model.SubmitResult{}, fmt.Errorf("moderation：寫入檢舉：%w", err)
	}
	if err := s.recordRequest(ctx, q, in, row.ID, hash); err != nil {
		return model.SubmitResult{}, err
	}
	if err := events.Publish(ctx, s.d.Events, model.ReportReceived{
		EventID: s.d.NewID(), OccurredAt: now, ReportID: row.ID, CaseID: c.ID, Target: in.Target,
		ReporterBuyerID: in.ReporterBuyerID, ReporterStoreID: in.ReporterStoreID, Category: in.Category,
	}); err != nil {
		return model.SubmitResult{}, err
	}
	return model.SubmitResult{Report: view.Report(row, nil), Case: view.Case(attached), Created: true}, nil
}

func buyerPtr(in model.SubmitInput) *int64 {
	if in.ReporterBuyerID > 0 {
		return &in.ReporterBuyerID
	}
	return nil
}

func storePtr(in model.SubmitInput) *int64 {
	if in.ReporterStoreID > 0 {
		return &in.ReporterStoreID
	}
	return nil
}

func (s *Service) bump(ctx context.Context, q *store.Queries, in model.SubmitInput, now time.Time) (int32, error) {
	if in.ReporterStoreID > 0 {
		return q.BumpStoreDailyCount(ctx, store.BumpStoreDailyCountParams{ReporterStoreID: &in.ReporterStoreID, Day: twtime.Day(now), DailyLimit: model.DailyReportLimit})
	}
	return q.BumpDailyCount(ctx, store.BumpDailyCountParams{ReporterBuyerID: &in.ReporterBuyerID, Day: twtime.Day(now), DailyLimit: model.DailyReportLimit})
}

func (s *Service) recordRequest(ctx context.Context, q *store.Queries, in model.SubmitInput, reportID uuid.UUID, hash []byte) error {
	if in.RequestID == nil {
		return nil
	}
	n, err := q.InsertReportRequest(ctx, store.InsertReportRequestParams{ReporterBuyerID: buyerPtr(in), ReporterStoreID: storePtr(in), RequestID: *in.RequestID, ReportID: reportID, RequestHash: hash})
	if err != nil {
		return fmt.Errorf("moderation：記下 request_id：%w", err)
	}
	if n == 0 {
		return errRetry
	}
	return nil
}

func requestHash(in model.SubmitInput) []byte {
	id := in.Target.ContentID.String()
	if in.Target.Kind == model.TargetBuyer {
		id = fmt.Sprint(in.Target.BuyerID)
	}
	return idempotency.Hash("report", string(in.Target.Kind), id, string(in.Category), in.Description)
}

// MyReports 是檢舉人自己的檢舉，新的在前（keyset，不回總數）；每筆的結果由所屬案件的決定推導。
func (s *Service) MyReports(ctx context.Context, buyerID int64, after *model.ReportCursor, size int32) (model.ReportPage, error) {
	size, err := pagination.PageSize(size, model.DefaultPageSize, model.MaxPageSize)
	if err != nil {
		return model.ReportPage{}, err
	}
	at, id := after.Key()
	rows, err := store.New(s.d.DB.Conn(ctx)).ListReportsOfReporter(ctx, store.ListReportsOfReporterParams{
		ReporterBuyerID: &buyerID, AfterCreated: at, AfterID: id, MaxRows: size + 1,
	})
	if err != nil {
		return model.ReportPage{}, fmt.Errorf("moderation：列出我的檢舉：%w", err)
	}
	rows, more := model.TrimPage(rows, size)
	items := make([]model.ReportItem, len(rows))
	for i, r := range rows {
		items[i] = reportItem(r.ModerationReport, r.TargetKind, r.ContentID, r.BuyerID, r.Decision, r.ClosedAt)
	}
	return reportPage(items, rows, more, func(r store.ListReportsOfReporterRow) store.ModerationReport { return r.ModerationReport }), nil
}

// MyStoreReports 是店家檢舉人自己的檢舉（只有聊天訊息），形狀同 MyReports。
func (s *Service) MyStoreReports(ctx context.Context, storeID int64, after *model.ReportCursor, size int32) (model.ReportPage, error) {
	size, err := pagination.PageSize(size, model.DefaultPageSize, model.MaxPageSize)
	if err != nil {
		return model.ReportPage{}, err
	}
	at, id := after.Key()
	rows, err := store.New(s.d.DB.Conn(ctx)).ListReportsOfStoreReporter(ctx, store.ListReportsOfStoreReporterParams{
		ReporterStoreID: &storeID, AfterCreated: at, AfterID: id, MaxRows: size + 1,
	})
	if err != nil {
		return model.ReportPage{}, fmt.Errorf("moderation：列出店家的檢舉：%w", err)
	}
	rows, more := model.TrimPage(rows, size)
	items := make([]model.ReportItem, len(rows))
	for i, r := range rows {
		items[i] = reportItem(r.ModerationReport, r.TargetKind, r.ContentID, r.BuyerID, r.Decision, r.ClosedAt)
	}
	return reportPage(items, rows, more, func(r store.ListReportsOfStoreReporterRow) store.ModerationReport { return r.ModerationReport }), nil
}

func reportItem(r store.ModerationReport, targetKind string, contentID *uuid.UUID, buyerID *int64, decision *string, closedAt *time.Time) model.ReportItem {
	t := model.Target{Kind: model.TargetKind(targetKind)}
	if contentID != nil {
		t.ContentID = *contentID
	}
	if buyerID != nil {
		t.BuyerID = *buyerID
	}
	return model.ReportItem{Report: view.Report(r, decision), Target: t, ClosedAt: closedAt}
}

func reportPage[R any](items []model.ReportItem, rows []R, more bool, report func(R) store.ModerationReport) model.ReportPage {
	out := model.ReportPage{Items: items}
	if more {
		last := report(rows[len(rows)-1])
		out.Next = &model.ReportCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return out
}

func (s *Service) replay(ctx context.Context, q *store.Queries, r store.ModerationReport) (model.SubmitResult, error) {
	row, err := q.GetCase(ctx, r.CaseID)
	if err != nil {
		return model.SubmitResult{}, fmt.Errorf("moderation：讀取案件 %s：%w", r.CaseID, err)
	}
	return model.SubmitResult{Report: view.Report(r, row.Decision), Case: view.Case(row)}, nil
}

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

func (s *Service) freeze(ctx context.Context, in model.SubmitInput) (snapshot.Content, error) {
	switch in.Target.Kind {
	case model.TargetBuyer:
		return s.freezeBuyer(ctx, in)
	case model.TargetReview:
		return s.freezeReview(ctx, in)
	case model.TargetChatMessage:
		return s.freezeChatMessage(ctx, in)
	case model.TargetReviewReply:
		return s.freezeReviewReply(ctx, in)
	case model.TargetPost, model.TargetComment, model.TargetForumEntry:
		return s.freezeSocial(ctx, in)
	}
	return snapshot.Content{}, apperr.InvalidInputOn([]string{"Target"}, "%s", moderationtext.ReportTargetInvalid(uitext.LocaleFrom(ctx)))
}

func (s *Service) freezeSocial(ctx context.Context, in model.SubmitInput) (snapshot.Content, error) {
	l := uitext.LocaleFrom(ctx)
	st := social.Target{Kind: social.TargetKind(in.Target.Kind), ID: in.Target.ContentID}
	snap, err := s.d.Social.ContentSnapshot(ctx, st)
	if err != nil {
		return snapshot.Content{}, err
	}
	if !snap.Exists || !snap.Readable {
		return snapshot.Content{}, apperr.NotFound("%s", moderationtext.ReportContentNotFound(l))
	}
	if err := s.notSelf(ctx, in, snap.Author); err != nil {
		return snapshot.Content{}, err
	}
	return snapshot.Content{Kind: in.Target.Kind, Social: &snapshot.Social{
		AuthorKind: string(snap.Author.Kind), AuthorID: snap.Author.ID, StoreID: snap.StoreID,
		Body: snap.Body, MediaRefs: snap.MediaRefs, AuthoredAt: snap.AuthoredAt, PostID: uuidString(snap.PostID), ThreadID: uuidString(snap.ThreadID),
	}}, nil
}

func (s *Service) notSelf(ctx context.Context, in model.SubmitInput, author social.Actor) error {
	same, err := s.sameAccount(ctx, in, author)
	if err != nil {
		return err
	}
	if same {
		return apperr.InvalidInputOn([]string{"Target"}, "%s", moderationtext.ReportSelfContent(uitext.LocaleFrom(ctx)))
	}
	return nil
}

func contentNotFound(ctx context.Context, err error) error {
	if apperr.KindOf(err) == apperr.KindNotFound {
		return apperr.NotFound("%s", moderationtext.ReportContentNotFound(uitext.LocaleFrom(ctx)))
	}
	return err
}

func (s *Service) freezeReview(ctx context.Context, in model.SubmitInput) (snapshot.Content, error) {
	r, err := s.d.Review.ReviewForReport(ctx, in.Target.ContentID, in.ReporterBuyerID)
	if err != nil {
		return snapshot.Content{}, contentNotFound(ctx, err)
	}
	if err := s.notSelf(ctx, in, social.Actor{Kind: social.ActorBuyer, ID: r.BuyerID}); err != nil {
		return snapshot.Content{}, err
	}
	out := snapshot.Review{
		AuthorID: r.BuyerID, SubjectStoreID: r.StoreID, ProductID: r.ProductID, Rating: r.Rating,
		Body: r.Content, MediaRefs: r.AssetIDs, AuthoredAt: r.CreatedAt,
	}
	if r.Reply != nil {
		out.ReplyBody = r.Reply.Content
	}
	return snapshot.Content{Kind: model.TargetReview, Review: &out}, nil
}

func (s *Service) freezeChatMessage(ctx context.Context, in model.SubmitInput) (snapshot.Content, error) {
	party := im.Party{Kind: im.PartyBuyer, BuyerID: in.ReporterBuyerID}
	if in.ReporterStoreID > 0 {
		st, err := s.d.Seller.Store(ctx, in.ReporterStoreID)
		if err != nil {
			return snapshot.Content{}, contentNotFound(ctx, err)
		}
		party = im.Party{Kind: im.PartyStore, Store: &st}
	}
	m, err := s.d.IM.MessageForReport(ctx, in.Target.ContentID, party, model.ChatContextWindow)
	if err != nil {
		return snapshot.Content{}, contentNotFound(ctx, err)
	}
	author := social.Actor{Kind: social.ActorBuyer, ID: m.Message.SenderID}
	var storeID int64
	if m.Message.SenderKind == im.PartyStore {
		author.Kind, storeID = social.ActorStore, m.Message.SenderID
	}
	if err := s.notSelf(ctx, in, author); err != nil {
		return snapshot.Content{}, err
	}
	out := snapshot.Chat{
		AuthorKind: string(author.Kind), AuthorID: author.ID, StoreID: storeID, Body: m.Message.Text, MediaRefs: assetRefs(m.Message.AssetID),
		AuthoredAt: m.Message.CreatedAt, ConversationID: m.ConversationID.String(), Seq: m.Message.Seq,
		Context: make([]snapshot.ContextMessage, len(m.Context)),
	}
	for i, c := range m.Context {
		out.Context[i] = snapshot.ContextMessage{
			Seq: c.Seq, SenderKind: string(c.SenderKind), SenderID: c.SenderID, Type: string(c.Type), Body: c.Text, MediaRefs: assetRefs(c.AssetID),
			SentAt: c.CreatedAt, Hidden: c.Hidden, Erased: c.Erased,
		}
	}
	return snapshot.Content{Kind: model.TargetChatMessage, Chat: &out}, nil
}

func (s *Service) freezeReviewReply(ctx context.Context, in model.SubmitInput) (snapshot.Content, error) {
	r, err := s.d.Review.ReviewForReport(ctx, in.Target.ContentID, in.ReporterBuyerID)
	if err != nil {
		return snapshot.Content{}, contentNotFound(ctx, err)
	}
	if r.Reply == nil {
		return snapshot.Content{}, apperr.NotFound("%s", moderationtext.ReportContentNotFound(uitext.LocaleFrom(ctx)))
	}
	if err := s.notSelf(ctx, in, social.Actor{Kind: social.ActorStore, ID: r.StoreID}); err != nil {
		return snapshot.Content{}, err
	}
	return snapshot.Content{Kind: model.TargetReviewReply, ReviewReply: &snapshot.ReviewReply{
		StoreID: r.StoreID, ReviewID: r.ID.String(), ReviewerID: r.BuyerID, Body: r.Reply.Content, AuthoredAt: r.Reply.RepliedAt,
	}}, nil
}

func assetRefs(id string) []string {
	if id == "" {
		return nil
	}
	return []string{id}
}

func (s *Service) freezeBuyer(ctx context.Context, in model.SubmitInput) (snapshot.Content, error) {
	displays, err := s.d.Buyer.BatchDisplay(ctx, []int64{in.Target.BuyerID})
	if err != nil {
		return snapshot.Content{}, err
	}
	if len(displays) != 1 || displays[0].Deleted {
		return snapshot.Content{}, apperr.NotFound("%s", moderationtext.ReportBuyerNotFound(uitext.LocaleFrom(ctx)))
	}
	same, err := s.sameAccount(ctx, in, social.Actor{Kind: social.ActorBuyer, ID: in.Target.BuyerID})
	if err != nil {
		return snapshot.Content{}, err
	}
	if same {
		return snapshot.Content{}, apperr.InvalidInputOn([]string{"Target"}, "%s", moderationtext.ReportSelfBuyer(uitext.LocaleFrom(ctx)))
	}
	c := snapshot.Buyer{}
	if displays[0].DisplayName != nil {
		c.DisplayName = *displays[0].DisplayName
	}
	if displays[0].AvatarAssetID != nil {
		c.AvatarAssetID = displays[0].AvatarAssetID.String()
	}
	return snapshot.Content{Kind: model.TargetBuyer, Buyer: &c}, nil
}

func (s *Service) reporterAccount(ctx context.Context, in model.SubmitInput) (int64, error) {
	if in.ReporterStoreID > 0 {
		st, err := s.d.Seller.Store(ctx, in.ReporterStoreID)
		if err != nil {
			return 0, err
		}
		return st.AccountID, nil
	}
	return s.d.Buyer.AccountIDByBuyerID(ctx, in.ReporterBuyerID)
}

func (s *Service) sameAccount(ctx context.Context, in model.SubmitInput, author social.Actor) (bool, error) {
	reporter, err := s.reporterAccount(ctx, in)
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

func uuidString(id uuid.UUID) string {
	if id == uuid.Nil() {
		return ""
	}
	return id.String()
}
