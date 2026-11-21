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
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
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
	"github.com/Will413028/hawkr/backend/internal/platform/idempotency"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// maxAttempts 是遇到並行競爭（案件在掛上檢舉前被結掉、同一位檢舉人並行重送）時整個 transaction 重來的上限。
const maxAttempts = 4

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
	Social *social.Module
	Review Reviews
	IM     Chats
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

// lockReporter 以 FOR KEY SHARE 鎖檢舉人的擁有者列：買家尚未關閉（刪除中放行）、店家尚未退場；否則回檢舉人專屬的 NotFound。
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

func (s *Service) attempt(ctx context.Context, in model.SubmitInput) (model.SubmitResult, error) {
	// 檢舉人的擁有者守門鎖先於 moderation 的任何列（buyer、seller 先於 moderation）：關閉或退場的訂閱者清檢舉人資料時，並行的送出與轉移在擁有者列上序列化。
	// 必要條件：店家檢舉人已退場時，新檢舉在凍結內容（CapChatRead）已回 NotFound，鎖要擋的是凍結之前就返回、卻會寫 report_requests 的路徑（併入進行中案件的 recordRequest）與重送；買家檢舉人沒有這道前置擋，鎖是唯一的守門。
	if err := s.lockReporter(ctx, in); err != nil {
		return model.SubmitResult{}, err
	}
	q := store.New(s.d.DB.Conn(ctx))
	var hash []byte
	if in.RequestID != nil {
		hash = requestHash(in)
		// request_id 先於一切（A7-D28 第 3 點）：同一位檢舉人的同一個 request_id 一律回第一次的結果，不看案件現在是進行中還是已結案。
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
		// 併入進行中案件裡已有的那一筆：也記下這個 request_id，結案後同一個 request_id 重送才仍回這一筆。
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
	body, sum := snapshot.Encode(content)
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
		ID: snapshotID, CaseID: c.ID, ContentSha256: sum[:], Content: body, AssetIds: content.AssetIDs(), CapturedAt: now,
	})
	if err != nil {
		return model.SubmitResult{}, fmt.Errorf("moderation：寫入快照：%w", err)
	}
	if n == 1 {
		if err := s.d.Holds.Hold(ctx, snapshotID, now, content.AssetIDs()); err != nil {
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

// bump 扣檢舉人當天的額度：買家與店家各自一份計數，上限相同（model.DailyReportLimit）；到上限回 pgx.ErrNoRows。
func (s *Service) bump(ctx context.Context, q *store.Queries, in model.SubmitInput, now time.Time) (int32, error) {
	if in.ReporterStoreID > 0 {
		return q.BumpStoreDailyCount(ctx, store.BumpStoreDailyCountParams{ReporterStoreID: &in.ReporterStoreID, Day: twtime.Day(now), DailyLimit: model.DailyReportLimit})
	}
	return q.BumpDailyCount(ctx, store.BumpDailyCountParams{ReporterBuyerID: &in.ReporterBuyerID, Day: twtime.Day(now), DailyLimit: model.DailyReportLimit})
}

// recordRequest 記下 request_id 對應的檢舉（沒帶 request_id 時什麼都不做）。同一個 (檢舉人, request_id) 已被並行的請求記下時回 errRetry：
// 這一輪的交易整個放棄，重試時從頭讀到對方的紀錄並比對內容雜湊。
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

// requestHash 是送出輸入（對象、類別、說明；已正規化）的 SHA-256：同一個 request_id 重送時比對內容是否相同的唯一定義。
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
	out := model.ReportPage{Items: make([]model.ReportItem, len(rows))}
	for i, r := range rows {
		t := model.Target{Kind: model.TargetKind(r.TargetKind)}
		if r.ContentID != nil {
			t.ContentID = *r.ContentID
		}
		if r.BuyerID != nil {
			t.BuyerID = *r.BuyerID
		}
		out.Items[i] = model.ReportItem{Report: view.Report(r.ModerationReport, r.Decision), Target: t, ClosedAt: r.ClosedAt}
	}
	if more {
		last := rows[len(rows)-1].ModerationReport
		out.Next = &model.ReportCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return out, nil
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

// freezeSocial 向 social 讀貼文、留言或討論區留言。
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

// notSelf 擋掉檢舉自己的內容（含同一帳號的其他身分）。
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

// contentNotFound 把擁有者模組的 NotFound（不存在、不可讀、對話不屬於檢舉人）統一成檢舉的 NotFound 句子，不洩漏原因；其他錯誤原樣回傳。
func contentNotFound(ctx context.Context, err error) error {
	if apperr.KindOf(err) == apperr.KindNotFound {
		return apperr.NotFound("%s", moderationtext.ReportContentNotFound(uitext.LocaleFrom(ctx)))
	}
	return err
}

// freezeReview 向 review 讀評價（可讀規則同顯示：公開可讀、觀看者封鎖），作者永遠是買家。
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

// freezeChatMessage 向 im 讀被檢舉的訊息與前後各 ChatContextWindow 則（一句 SQL）；檢舉人要是對話的一方，訊息已隱藏或抹除都是 NotFound。
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

// freezeReviewReply 向 review 讀評價與店家回覆（可讀規則同評價；回覆被下架或沒有回覆都是 NotFound），作者是評價所屬的店家。
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
	if displays[0].Avatar != nil {
		c.Avatar = *displays[0].Avatar
	}
	return snapshot.Content{Kind: model.TargetBuyer, Buyer: &c}, nil
}

// reporterAccount 是檢舉人的帳號：買家檢舉人取買家的帳號，店家檢舉人取店家所屬的帳號。
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

// sameAccount 說明作者是不是檢舉人自己：買家作者比對帳號（同一個帳號的歷史 stint 也算自己），店家作者比對店家所屬的帳號。
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

// uuidString 回傳 id 的字串；零值是空字串（快照 JSON 不輸出）。
func uuidString(id uuid.UUID) string {
	if id == uuid.Nil() {
		return ""
	}
	return id.String()
}
