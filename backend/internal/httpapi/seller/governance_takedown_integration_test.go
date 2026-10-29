//go:build integration

package seller_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"

	adminsellerpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

type seededContent struct {
	post, comment, entry                  uuid.UUID
	buyerComment, deletedPost, deletedCmt uuid.UUID
	buyerEntry, deletedEntry              uuid.UUID
}

// seedContent 直接寫 social 的列（店家身分的貼文、留言、討論區留言）：這組測試要的是內容存在而且屬於這家店，不經 social 的寫入規則。
func (f *adminFixture) seedContent(t *testing.T, storeID int64) seededContent {
	t.Helper()
	c := seededContent{post: uuid.NewV7(), comment: uuid.NewV7(), entry: uuid.NewV7(), buyerComment: uuid.NewV7(), deletedPost: uuid.NewV7(),
		deletedCmt: uuid.NewV7(), buyerEntry: uuid.NewV7(), deletedEntry: uuid.NewV7()}
	now := time.Now()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO social.posts (id, store_id, body, status, published_at, created_at, updated_at) VALUES ($1, $2, '貼文', 'PUBLISHED', $3, $3, $3)`, []any{c.post, storeID, now}},
		{`INSERT INTO social.comments (id, post_id, actor_kind, actor_id, body, created_at, updated_at) VALUES ($1, $2, 'STORE', $3, '留言', $4, $4)`, []any{c.comment, c.post, storeID, now}},
		{`INSERT INTO social.forum_entries (id, store_id, actor_kind, actor_id, body, created_at, updated_at) VALUES ($1, $2, 'STORE', $2, '討論', $3, $3)`, []any{c.entry, storeID, now}},
		{`INSERT INTO social.comments (id, post_id, actor_kind, actor_id, body, created_at, updated_at) VALUES ($1, $2, 'BUYER', 7, '買家的留言', $3, $3)`, []any{c.buyerComment, c.post, now}},
		{`INSERT INTO social.forum_entries (id, store_id, actor_kind, actor_id, body, created_at, updated_at) VALUES ($1, $2, 'BUYER', 7, '買家的討論', $3, $3)`, []any{c.buyerEntry, storeID, now}},
		{`INSERT INTO social.posts (id, store_id, body, status, published_at, created_at, updated_at, deleted_at) VALUES ($1, $2, '已刪除的貼文', 'PUBLISHED', $3, $3, $3, $3)`, []any{c.deletedPost, storeID, now}},
		{`INSERT INTO social.comments (id, post_id, actor_kind, actor_id, body, created_at, updated_at, deleted_at) VALUES ($1, $2, 'STORE', $3, NULL, $4, $4, $4)`, []any{c.deletedCmt, c.post, storeID, now}},
		{`INSERT INTO social.forum_entries (id, store_id, actor_kind, actor_id, body, created_at, updated_at, deleted_at) VALUES ($1, $2, 'STORE', $2, NULL, $3, $3, $3)`, []any{c.deletedEntry, storeID, now}},
	} {
		if _, err := f.db.Conn(context.Background()).Exec(context.Background(), q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	return c
}

func (f *adminFixture) takedownRequest(storeID int64, refs ...*pb.ContentRef) *adminsellerpb.CreateMeasureRequest {
	return &adminsellerpb.CreateMeasureRequest{
		RequestId: uuid.New().String(), StoreId: sid(storeID), Statement: "內容違規", Note: "內部紀錄",
		Measure: &adminsellerpb.CreateMeasureRequest_ContentTakedown{ContentTakedown: &adminsellerpb.ContentTakedownInput{
			Contents: refs,
			Violation: &adminsellerpb.ViolationBasis{Finding: &adminsellerpb.NewFinding{
				Category: commonpb.PolicyCategory_POLICY_CATEGORY_SPAM, Severity: pb.ViolationSeverity_VIOLATION_SEVERITY_LOW}},
		}},
	}
}

func fieldsOf(err error) []string {
	var fields []string
	if ce := (*connect.Error)(nil); errors.As(err, &ce) {
		for _, d := range ce.Details() {
			if v, e := d.Value(); e == nil {
				if br, ok := v.(*errdetails.BadRequest); ok {
					for _, fv := range br.GetFieldViolations() {
						fields = append(fields, fv.GetField())
					}
				}
			}
		}
	}
	return fields
}

func (f *adminFixture) count(t *testing.T, sql string, args ...any) int64 {
	t.Helper()
	var n int64
	f.scan(t, &n, sql, args...)
	return n
}

// 內容下架由 social 的同 transaction 訂閱者核對：別家店的內容、買家寫的內容、已刪除與不存在的內容都被拒絕，欄位路徑指到那一則，措施與複本都不存在。
func TestCreateContentTakedownRejectsContentOfAnotherStoreOrMissing(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	_, target := f.newSeller(t, "被處分的店")
	_, other := f.newSeller(t, "別家的店")
	mine, theirs := f.seedContent(t, target.StoreID), f.seedContent(t, other.StoreID)
	for name, tc := range map[string]struct {
		refs []*pb.ContentRef
		want string
	}{
		"別家店的貼文":       {[]*pb.ContentRef{{Kind: pb.ContentKind_CONTENT_KIND_POST, Id: mine.post.String()}, {Kind: pb.ContentKind_CONTENT_KIND_POST, Id: theirs.post.String()}}, "content_takedown.contents[1]"},
		"別家店的留言":       {[]*pb.ContentRef{{Kind: pb.ContentKind_CONTENT_KIND_COMMENT, Id: theirs.comment.String()}}, "content_takedown.contents[0]"},
		"別家店的討論":       {[]*pb.ContentRef{{Kind: pb.ContentKind_CONTENT_KIND_FORUM_ENTRY, Id: theirs.entry.String()}}, "content_takedown.contents[0]"},
		"不存在的貼文":       {[]*pb.ContentRef{{Kind: pb.ContentKind_CONTENT_KIND_POST, Id: uuid.NewV7().String()}}, "content_takedown.contents[0]"},
		"種類對不上":        {[]*pb.ContentRef{{Kind: pb.ContentKind_CONTENT_KIND_COMMENT, Id: mine.post.String()}}, "content_takedown.contents[0]"},
		"買家在這家店貼文下的留言": {[]*pb.ContentRef{{Kind: pb.ContentKind_CONTENT_KIND_COMMENT, Id: mine.buyerComment.String()}}, "content_takedown.contents[0]"},
		"買家的討論區留言":     {[]*pb.ContentRef{{Kind: pb.ContentKind_CONTENT_KIND_POST, Id: mine.post.String()}, {Kind: pb.ContentKind_CONTENT_KIND_FORUM_ENTRY, Id: mine.buyerEntry.String()}}, "content_takedown.contents[1]"},
		"已刪除的貼文":       {[]*pb.ContentRef{{Kind: pb.ContentKind_CONTENT_KIND_POST, Id: mine.deletedPost.String()}}, "content_takedown.contents[0]"},
		"已刪除的留言":       {[]*pb.ContentRef{{Kind: pb.ContentKind_CONTENT_KIND_COMMENT, Id: mine.deletedCmt.String()}}, "content_takedown.contents[0]"},
		"已刪除的討論區留言":    {[]*pb.ContentRef{{Kind: pb.ContentKind_CONTENT_KIND_FORUM_ENTRY, Id: mine.deletedEntry.String()}}, "content_takedown.contents[0]"},
	} {
		_, err := f.governance.CreateMeasure(ctx, connect.NewRequest(f.takedownRequest(target.StoreID, tc.refs...)))
		if connect.CodeOf(err) != connect.CodeInvalidArgument || !slices.Equal(fieldsOf(err), []string{tc.want}) {
			t.Fatalf("%s：err=%v fields=%v，預期 invalid_argument 指 %s", name, err, fieldsOf(err), tc.want)
		}
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measures"); n != 0 {
		t.Fatalf("被拒絕的請求不該建立措施：%d", n)
	}
	if n := f.count(t, "SELECT count(*) FROM social.content_takedowns"); n != 0 {
		t.Fatalf("被拒絕的請求不該寫複本：%d", n)
	}
}

// 經 Connect 建立：social 的複本與旗標在同一個 transaction 改變；撤銷措施後複本刪除、旗標恢復。
func TestContentTakedownThroughConnectWritesAndRemovesTheSocialCopy(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	_, s := f.newSeller(t, "有貼文的店")
	c := f.seedContent(t, s.StoreID)
	created, err := f.governance.CreateMeasure(ctx, connect.NewRequest(f.takedownRequest(s.StoreID,
		&pb.ContentRef{Kind: pb.ContentKind_CONTENT_KIND_POST, Id: c.post.String()},
		&pb.ContentRef{Kind: pb.ContentKind_CONTENT_KIND_COMMENT, Id: c.comment.String()},
		&pb.ContentRef{Kind: pb.ContentKind_CONTENT_KIND_FORUM_ENTRY, Id: c.entry.String()})))
	if err != nil {
		t.Fatal(err)
	}
	if n := f.count(t, "SELECT count(*) FROM social.content_takedowns WHERE measure_id = $1", created.Msg.GetMeasure().GetId()); n != 3 {
		t.Fatalf("複本 %d 列，預期 3", n)
	}
	if n := f.count(t, "SELECT count(*) FROM social.comments WHERE id = $1 AND NOT counted", c.comment); n != 1 {
		t.Fatal("留言的旗標要同 transaction 變成不計入")
	}
	if n := f.count(t, "SELECT count(*) FROM social.forum_entries WHERE id = $1 AND NOT counted", c.entry); n != 1 {
		t.Fatal("討論區留言的旗標要同 transaction 變成不計入")
	}
	id := uuid.MustParse(created.Msg.GetMeasure().GetId())
	if _, err := f.module.RevokeMeasure(ctx, seller.RevokeMeasureInput{AdminID: f.adminID, MeasureID: id, ExpectedVersion: 1, Reason: seller.RevokeAdminCorrection}); err != nil {
		t.Fatal(err)
	}
	if n := f.count(t, "SELECT count(*) FROM social.content_takedowns"); n != 0 {
		t.Fatalf("撤銷後複本 %d 列，預期 0", n)
	}
	if n := f.count(t, "SELECT count(*) FROM social.comments WHERE id = $1 AND counted", c.comment); n != 1 {
		t.Fatal("撤銷後留言的旗標恢復")
	}
	if n := f.count(t, "SELECT count(*) FROM social.forum_entries WHERE id = $1 AND counted", c.entry); n != 1 {
		t.Fatal("撤銷後討論區留言的旗標恢復")
	}
}

// 呼叫方的 transaction 回滾：措施與 social 的複本都不存在（CreateMeasureInTx 加入呼叫方的 transaction，social 的訂閱者在同一個 transaction）。
func TestContentTakedownRollsBackWithTheCallersTransaction(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	_, s := f.newSeller(t, "有貼文的店")
	c := f.seedContent(t, s.StoreID)
	finding := seller.ViolationBasis{Finding: &seller.NewFinding{Category: policy.Spam, Severity: seller.SeverityLow}}
	errRollback := errors.New("rollback")
	err := f.db.InTx(ctx, func(ctx context.Context) error {
		if _, err := f.module.CreateMeasureInTx(ctx, seller.CreateMeasureInput{
			AdminID: f.adminID, StoreID: s.StoreID, Kind: seller.MeasureContentTakedown, StatementToSeller: "內容違規", Note: "內部紀錄", Violation: &finding,
			Takedown: &seller.ContentTakedown{Contents: []seller.ContentRef{{Kind: seller.ContentPost, ID: c.post}, {Kind: seller.ContentComment, ID: c.comment}}},
		}); err != nil {
			return err
		}
		if n := f.countIn(ctx, "SELECT count(*) FROM social.content_takedowns"); n != 2 {
			t.Errorf("同 transaction 內應已寫入複本：%d", n)
		}
		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatal(err)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measures"); n != 0 {
		t.Fatalf("回滾後措施還在：%d", n)
	}
	if n := f.count(t, "SELECT count(*) FROM social.content_takedowns"); n != 0 {
		t.Fatalf("回滾後複本還在：%d", n)
	}
	if n := f.count(t, "SELECT count(*) FROM social.comments WHERE id = $1 AND counted", c.comment); n != 1 {
		t.Fatal("回滾後留言的旗標不該變")
	}
}

func (f *adminFixture) countIn(ctx context.Context, sql string) int64 {
	var n int64
	if err := f.db.Conn(ctx).QueryRow(ctx, sql).Scan(&n); err != nil {
		return -1
	}
	return n
}

// 不經 admin handler 的入口（CreateMeasureInTx，之後的 moderation）一樣被擋：歸屬核對在 social 的訂閱者，不在 handler。
func TestContentTakedownThroughCreateInTxRejectsContentThatIsNotTheStoresOwn(t *testing.T) {
	t.Parallel()
	f := setupAdmin(t)
	ctx := context.Background()
	_, target := f.newSeller(t, "被處分的店")
	_, other := f.newSeller(t, "別家的店")
	mine, theirs := f.seedContent(t, target.StoreID), f.seedContent(t, other.StoreID)
	finding := seller.ViolationBasis{Finding: &seller.NewFinding{Category: policy.Spam, Severity: seller.SeverityLow}}
	for name, ref := range map[string]seller.ContentRef{
		"別家店的貼文":  {Kind: seller.ContentPost, ID: theirs.post},
		"買家的留言":   {Kind: seller.ContentComment, ID: mine.buyerComment},
		"已刪除的貼文":  {Kind: seller.ContentPost, ID: mine.deletedPost},
		"不存在的討論區": {Kind: seller.ContentForumEntry, ID: uuid.NewV7()},
	} {
		err := f.db.InTx(ctx, func(ctx context.Context) error {
			_, err := f.module.CreateMeasureInTx(ctx, seller.CreateMeasureInput{
				AdminID: f.adminID, StoreID: target.StoreID, Kind: seller.MeasureContentTakedown, StatementToSeller: "內容違規", Note: "內部紀錄", Violation: &finding,
				Takedown: &seller.ContentTakedown{Contents: []seller.ContentRef{ref}},
			})
			return err
		})
		if err == nil {
			t.Fatalf("%s：應被拒絕", name)
		}
	}
	if n := f.count(t, "SELECT count(*) FROM seller.measures"); n != 0 {
		t.Fatalf("被拒絕後不該有措施：%d", n)
	}
	if n := f.count(t, "SELECT count(*) FROM social.content_takedowns"); n != 0 {
		t.Fatalf("被拒絕後不該有複本：%d", n)
	}
}
