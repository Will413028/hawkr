// Package engage 是讚、瀏覽與分享。計數由資料庫的 trigger 依實際新增或刪除的列加減（db/objects.sql），這裡只寫集合的列：
// 並發雙擊只有一列（INSERT ... ON CONFLICT DO NOTHING）、只加一（SD17）；瀏覽與分享先寫去重列、真的新增才計（SD18）。
package engage

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"strconv"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/actor"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/reads"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/text"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/visibility"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Deps 是互動服務的依賴。
type Deps struct {
	DB     *db.DB
	Events *events.Bus
	Actors actor.Resolver
	Now    func() time.Time
}

// Service 是讚、瀏覽與分享的 use case。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

// LikePost 對公開可讀的貼文按讚（只有買家）；已經按過回 false、不發事件。PostLiked 只在真的新增一列時發。
func (s *Service) LikePost(ctx context.Context, buyerID int64, postID uuid.UUID) (bool, error) {
	now := s.d.Now().UTC()
	var added bool
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		post, err := reads.One[reads.PostRow](ctx, s.d.DB.Conn(ctx), reads.GetPublicPost(postID, now, visibility.Viewer{BuyerID: buyerID}))
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("%s", text.PostNotFound(uitext.LocaleFrom(ctx)))
		}
		if err != nil {
			return fmt.Errorf("social：讀取貼文 %s：%w", postID, err)
		}
		if err := s.d.Actors.ForBuyerWrite(ctx, buyerID, post.StoreID); err != nil {
			return err
		}
		n, err := q.InsertPostLike(ctx, store.InsertPostLikeParams{PostID: postID, BuyerID: buyerID, Now: now})
		if err != nil {
			return fmt.Errorf("social：按讚：%w", err)
		}
		if added = n == 1; !added {
			return nil
		}
		return events.Publish(ctx, s.d.Events, model.PostLiked{
			EventID: uuid.NewV7(), OccurredAt: now, PostID: postID, StoreID: post.StoreID, BuyerID: buyerID,
		})
	})
	return added, err
}

// UnlikePost 取消讚；沒有按過回 false。不要求貼文仍然公開可讀（內容被隱藏之後仍然可以收回自己的讚）。
func (s *Service) UnlikePost(ctx context.Context, buyerID int64, postID uuid.UUID) (bool, error) {
	if buyerID <= 0 {
		return false, apperr.PermissionDenied("%s", text.CallerBuyerRequired(uitext.LocaleFrom(ctx)))
	}
	n, err := s.q(ctx).DeletePostLike(ctx, store.DeletePostLikeParams{PostID: postID, BuyerID: buyerID})
	if err != nil {
		return false, fmt.Errorf("social：取消讚：%w", err)
	}
	return n == 1, nil
}

// PostsLikedBy 回傳 buyerID 按過讚的貼文（批次，給列表的個人化）。
func (s *Service) PostsLikedBy(ctx context.Context, buyerID int64, postIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	out := map[uuid.UUID]bool{}
	if buyerID <= 0 || len(postIDs) == 0 {
		return out, nil
	}
	ids, err := s.q(ctx).PostsLikedBy(ctx, store.PostsLikedByParams{BuyerID: buyerID, PostIds: postIDs})
	if err != nil {
		return nil, fmt.Errorf("social：讀取按讚：%w", err)
	}
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}

// LikeComment 對公開可讀的留言按讚。互動者依貼文歸屬推導：呼叫者此刻的店擁有貼文時是店家（要有 CapMarketing），否則是買家。
func (s *Service) LikeComment(ctx context.Context, c model.Caller, commentID uuid.UUID) (bool, error) {
	now := s.d.Now().UTC()
	var added bool
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		target, err := q.GetComment(ctx, commentID)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("%s", text.CommentNotFound(uitext.LocaleFrom(ctx)))
		}
		if err != nil {
			return fmt.Errorf("social：讀取留言 %s：%w", commentID, err)
		}
		storeID := target.PostStoreID
		_, err = reads.One[reads.StoreIDRow](ctx, s.d.DB.Conn(ctx), reads.ReadableCommentPost(commentID, now, actor.ViewerFor(c, storeID)))
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("%s", text.CommentNotFound(uitext.LocaleFrom(ctx)))
		}
		if err != nil {
			return fmt.Errorf("social：讀取留言 %s：%w", commentID, err)
		}
		a, err := s.d.Actors.ForWrite(ctx, c, storeID, seller.CapMarketing)
		if err != nil {
			return err
		}
		n, err := q.InsertCommentLike(ctx, store.InsertCommentLikeParams{CommentID: commentID, ActorKind: string(a.Kind), ActorID: a.ID, Now: now})
		if err != nil {
			return fmt.Errorf("social：留言按讚：%w", err)
		}
		added = n == 1
		return nil
	})
	return added, err
}

// UnlikeComment 取消留言的讚；沒有按過回 false。
func (s *Service) UnlikeComment(ctx context.Context, c model.Caller, commentID uuid.UUID) (bool, error) {
	q := s.q(ctx)
	row, err := q.GetComment(ctx, commentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, apperr.NotFound("%s", text.CommentNotFound(uitext.LocaleFrom(ctx)))
	}
	if err != nil {
		return false, fmt.Errorf("social：讀取留言 %s：%w", commentID, err)
	}
	a, ok := actor.ForViewer(c, row.PostStoreID)
	if !ok {
		return false, apperr.PermissionDenied("%s", text.CallerLoginRequired(uitext.LocaleFrom(ctx)))
	}
	n, err := q.DeleteCommentLike(ctx, store.DeleteCommentLikeParams{CommentID: commentID, ActorKind: string(a.Kind), ActorID: a.ID})
	if err != nil {
		return false, fmt.Errorf("social：取消留言的讚：%w", err)
	}
	return n == 1, nil
}

// CommentsLikedBy 回傳互動者按過讚的留言（批次）。
func (s *Service) CommentsLikedBy(ctx context.Context, a model.Actor, commentIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	out := map[uuid.UUID]bool{}
	if a.ID <= 0 || len(commentIDs) == 0 {
		return out, nil
	}
	ids, err := s.q(ctx).CommentsLikedBy(ctx, store.CommentsLikedByParams{ActorKind: string(a.Kind), ActorID: a.ID, CommentIds: commentIDs})
	if err != nil {
		return nil, fmt.Errorf("social：讀取留言的讚：%w", err)
	}
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}

// RecordShare 記一次分享（只有買家；同一位買家同一篇只算一次）。貼文要公開可讀；已經分享過回 false。
func (s *Service) RecordShare(ctx context.Context, buyerID int64, postID uuid.UUID) (bool, error) {
	now := s.d.Now().UTC()
	var added bool
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		if err := s.d.Actors.ForSelfService(ctx, buyerID); err != nil {
			return err
		}
		q := s.q(ctx)
		ok, err := reads.Flag(ctx, s.d.DB.Conn(ctx), reads.PostIsPublic(postID, now, visibility.Viewer{BuyerID: buyerID}))
		if err != nil {
			return fmt.Errorf("social：讀取貼文 %s：%w", postID, err)
		}
		if !ok {
			return apperr.NotFound("%s", text.PostNotFound(uitext.LocaleFrom(ctx)))
		}
		n, err := q.InsertPostShare(ctx, store.InsertPostShareParams{PostID: postID, BuyerID: buyerID, Now: now})
		if err != nil {
			return fmt.Errorf("social：記錄分享：%w", err)
		}
		added = n == 1
		return nil
	})
	return added, err
}

// Viewer 是瀏覽貼文的人：已登入的帳號（AccountID > 0），或匿名的 IP 與 User-Agent。BuyerID 是已登入者的買家身分（不是買家就是 0），
// 公開可讀的判斷用它套個人封鎖。
type Viewer struct {
	AccountID int64
	BuyerID   int64
	IP        string
	UserAgent string
}

// RecordView 記一次瀏覽：只計公開可讀的貼文；(貼文, 台北日, 觀看者鍵) 插入成功才算（trigger 把瀏覽數加一）。觀看者鍵是以當日 salt
// 做的 HMAC-SHA256（登入者是帳號，匿名是貼文 + IP + User-Agent），表裡沒有可回推的帳號或 IP；匿名沒有 IP 時不計數。店主與代行的讀取
// 由呼叫方不呼叫。回傳這次有沒有計入。觀看者封鎖了這家店時，貼文對他不公開可讀，不計入；已計入的瀏覽數不因封鎖回頭改變。
func (s *Service) RecordView(ctx context.Context, postID uuid.UUID, v Viewer) (bool, error) {
	if v.AccountID <= 0 && v.IP == "" {
		return false, nil
	}
	now := s.d.Now().UTC()
	day := twtime.Day(now)
	var counted bool
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		ok, err := reads.Flag(ctx, s.d.DB.Conn(ctx), reads.PostIsPublic(postID, now, visibility.Viewer{BuyerID: v.BuyerID}))
		if err != nil {
			return fmt.Errorf("social：讀取貼文 %s：%w", postID, err)
		}
		if !ok {
			return nil
		}
		salt, err := s.salt(ctx, q, day)
		if err != nil {
			return err
		}
		n, err := q.InsertPostView(ctx, store.InsertPostViewParams{PostID: postID, ViewDay: day, ViewerKey: viewerKey(salt, postID, v)})
		if err != nil {
			return fmt.Errorf("social：記錄瀏覽：%w", err)
		}
		counted = n == 1
		return nil
	})
	return counted, err
}

// salt 讀當日的 salt；第一次需要時寫入（ON CONFLICT DO NOTHING 後讀回，跨實例一致）。
func (s *Service) salt(ctx context.Context, q *store.Queries, day time.Time) ([]byte, error) {
	salt, err := q.GetViewSalt(ctx, day)
	if err == nil {
		return salt, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("social：讀取瀏覽的 salt：%w", err)
	}
	fresh := make([]byte, 32)
	if _, err := rand.Read(fresh); err != nil {
		return nil, fmt.Errorf("social：產生瀏覽的 salt：%w", err)
	}
	if err := q.InsertViewSalt(ctx, store.InsertViewSaltParams{ViewDay: day, Salt: fresh}); err != nil {
		return nil, fmt.Errorf("social：寫入瀏覽的 salt：%w", err)
	}
	salt, err = q.GetViewSalt(ctx, day)
	if err != nil {
		return nil, fmt.Errorf("social：讀取瀏覽的 salt：%w", err)
	}
	return salt, nil
}

// viewerKey 是 HMAC-SHA256(當日 salt, 訊息)：登入者 "a:" ‖ account_id；匿名 "v:" ‖ post_id ‖ ip ‖ ua（以 NUL 分隔，不會因串接而混淆）。
func viewerKey(salt []byte, postID uuid.UUID, v Viewer) []byte {
	mac := hmac.New(sha256.New, salt)
	if v.AccountID > 0 {
		mac.Write([]byte("a:" + strconv.FormatInt(v.AccountID, 10)))
	} else {
		mac.Write([]byte("v:" + postID.String() + "\x00" + v.IP + "\x00" + v.UserAgent))
	}
	return mac.Sum(nil)
}
