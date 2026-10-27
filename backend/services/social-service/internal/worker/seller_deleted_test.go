package worker

import (
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/media"
	"github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/social-service/internal/domain"
)

type mockPostRepoForSeller struct {
	domain.PostRepository
	softDeleteBySellerIDFn  func(ctx context.Context, sellerID int64) error
	listPostIDsBySellerIDFn func(ctx context.Context, sellerID int64) ([]int64, error)
}

func (m *mockPostRepoForSeller) SoftDeleteBySellerID(ctx context.Context, sellerID int64) error {
	return m.softDeleteBySellerIDFn(ctx, sellerID)
}

func (m *mockPostRepoForSeller) ListPostIDsBySellerID(ctx context.Context, sellerID int64) ([]int64, error) {
	return m.listPostIDsBySellerIDFn(ctx, sellerID)
}

type mockCommentRepoForSeller struct {
	domain.CommentRepository
	deleteLikesByUserIDFn func(ctx context.Context, kind int16, userID int64) error
}

func (m *mockCommentRepoForSeller) DeleteLikesByActor(ctx context.Context, kind int16, userID int64) error {
	return m.deleteLikesByUserIDFn(ctx, kind, userID)
}

func validSellerDeletedEvent(accountID, storeID int64) []byte {
	event := mq.SellerDeletedEvent{
		BaseEvent: mq.NewBaseEvent(mq.EventTypeSellerDeleted, "test"),
		AccountID: accountID,
		StoreID:   storeID,
	}
	b, _ := json.Marshal(event)
	return b
}

// noopSellerRepos returns post/comment repo mocks whose stubs fail the test if
// called — used by tests that expect the handler to short-circuit before
// reaching them (malformed JSON / zero store_id).
func noopSellerRepos(t *testing.T) (*mockPostRepoForSeller, *mockCommentRepoForSeller) {
	t.Helper()
	noop := func(context.Context, int64) error { t.Fatal("should not be called"); return nil }
	noopLikes := func(context.Context, int16, int64) error { t.Fatal("should not be called"); return nil }
	noopList := func(context.Context, int64) ([]int64, error) { t.Fatal("should not be called"); return nil, nil }
	return &mockPostRepoForSeller{softDeleteBySellerIDFn: noop, listPostIDsBySellerIDFn: noopList},
		&mockCommentRepoForSeller{deleteLikesByUserIDFn: noopLikes}
}

// fakeMediaRefClient 記錄 mediaReleaser 的呼叫。它多留一支 LinkImages 直接 panic：
// 介面本身已經不含這支（帳號終止只釋放，不會連上任何新引用），這裡保留它是為了讓
// 「有人把 handler 的參數放寬回完整 media client 再呼叫 Link」這條路徑當場炸掉。
type fakeMediaRefClient struct {
	releaseErr error
	calls      int
	gotRefType string
	gotRefIDs  []string
	gotKeep    []string
	// gotBatches 逐次記下每一批的 ref_ids（gotRefIDs 只留最後一次），供分批測試斷言
	// 切法本身，而不只是「總共被呼叫幾次」。
	gotBatches [][]string
	// gotKeeps 同理逐次記下每一批的 keepAssetIDs（gotKeep 只留最後一次）。「每一批都必須
	// 是釋放全部語意」要斷言的是每一批，只看最後一批的話，前面任何一批帶了 keep 名單
	// （＝那批貼文的圖片被保住、永遠不會被 GC 回收）測試都照樣綠。
	gotKeeps [][]string
	// failOnCall > 0 時只讓第 N 次呼叫回 releaseErr，其餘成功；用來測「中間某一批失敗」。
	// 0 代表沿用舊語意：releaseErr 非 nil 就每次都失敗。
	failOnCall int
	// seq 與被測 repo mock 共用同一個計數器，讓「釋放排在軟刪之後」是真的被觀察到，
	// 而不是靠測試自己在 handler 返回後補記一個必然更大的數字。
	seq          *int
	releaseOrder int
}

func (f *fakeMediaRefClient) LinkImages(context.Context, string, string, []string) error {
	panic("LinkImages must not be called by the seller.deleted handler")
}

func (f *fakeMediaRefClient) ReleaseReferences(_ context.Context, refType string, refIDs, keepAssetIDs []string) error {
	f.calls++
	if f.seq != nil {
		*f.seq++
		f.releaseOrder = *f.seq
	}
	f.gotRefType, f.gotRefIDs, f.gotKeep = refType, refIDs, keepAssetIDs
	f.gotBatches = append(f.gotBatches, append([]string(nil), refIDs...))
	f.gotKeeps = append(f.gotKeeps, append([]string(nil), keepAssetIDs...))
	if f.failOnCall > 0 && f.calls != f.failOnCall {
		return nil
	}
	return f.releaseErr
}

// okSellerRepos 是「四個操作全部成功」的預設組合，供只關心媒體釋放那一步的測試使用。
func okSellerRepos(postIDs []int64) (*mockPostRepoForSeller, *mockCommentRepoForSeller) {
	return &mockPostRepoForSeller{
			softDeleteBySellerIDFn:  func(context.Context, int64) error { return nil },
			listPostIDsBySellerIDFn: func(context.Context, int64) ([]int64, error) { return postIDs, nil },
		},
		&mockCommentRepoForSeller{deleteLikesByUserIDFn: func(context.Context, int16, int64) error { return nil }}
}

func TestSellerDeletedHandler_Success(t *testing.T) {
	var gotSellerID, commentLikeSellerID int64
	var commentLikeKind int16
	post := &mockPostRepoForSeller{
		softDeleteBySellerIDFn:  func(_ context.Context, sid int64) error { gotSellerID = sid; return nil },
		listPostIDsBySellerIDFn: func(context.Context, int64) ([]int64, error) { return nil, nil },
	}
	comment := &mockCommentRepoForSeller{
		deleteLikesByUserIDFn: func(_ context.Context, kind int16, sid int64) error {
			commentLikeKind, commentLikeSellerID = kind, sid
			return nil
		},
	}

	handler := newSellerDeletedHandler(post, comment, &fakeMediaRefClient{}, zap.NewNop())
	// accountID (100) deliberately != storeID (7487768729113814280): asserts
	// the handler keys off StoreID, not AccountID.
	if err := handler(context.Background(), "", validSellerDeletedEvent(100, 7487768729113814280)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotSellerID != 7487768729113814280 {
		t.Errorf("postRepo.SoftDeleteBySellerID got %d, want 7487768729113814280 (storeId)", gotSellerID)
	}
	if commentLikeSellerID != 7487768729113814280 {
		t.Errorf("commentRepo.DeleteLikesByActor got %d, want 7487768729113814280 (storeId)", commentLikeSellerID)
	}
	// 只清店家身分的讚；同號的買家讚不屬於這家店，貼文讚永遠是買家身分、完全不碰
	// （post mock 沒有 DeleteLikesByActor，被呼叫就 panic）。
	if commentLikeKind != domain.ActorKindStore {
		t.Errorf("comment like cleanup used actor kind %d, want %d (store)", commentLikeKind, domain.ActorKindStore)
	}
}

func TestSellerDeletedHandler_MalformedJSON(t *testing.T) {
	post, comment := noopSellerRepos(t)
	handler := newSellerDeletedHandler(post, comment, &fakeMediaRefClient{}, zap.NewNop())

	err := handler(context.Background(), "", []byte("not json"))
	if err == nil {
		t.Fatal("malformed JSON should return err (framework retries → DLQ; 36 §1.5)")
	}
	if !strings.Contains(err.Error(), "unmarshal event") {
		t.Errorf("error = %q, want it to come from mq.BindTypedHandler's wrapped message (\"unmarshal event...\")", err.Error())
	}
}

func TestSellerDeletedHandler_ZeroStoreID(t *testing.T) {
	post, comment := noopSellerRepos(t)
	handler := newSellerDeletedHandler(post, comment, &fakeMediaRefClient{}, zap.NewNop())
	// accountId present but store_id zero — must still reject, since store_id
	// is the field actually used for the soft-delete query.
	if err := handler(context.Background(), "", validSellerDeletedEvent(100, 0)); err == nil {
		t.Fatal("zero store_id should return err (framework retries → DLQ; 36 §1.5)")
	}
}

func TestSellerDeletedHandler_PostRepoError(t *testing.T) {
	dbErr := errors.New("db connection refused")
	post := &mockPostRepoForSeller{
		softDeleteBySellerIDFn:  func(context.Context, int64) error { return dbErr },
		listPostIDsBySellerIDFn: func(context.Context, int64) ([]int64, error) { return []int64{1}, nil },
	}
	comment := &mockCommentRepoForSeller{deleteLikesByUserIDFn: func(context.Context, int16, int64) error { t.Fatal("should not be called"); return nil }}
	mediaRef := &fakeMediaRefClient{}
	handler := newSellerDeletedHandler(post, comment, mediaRef, zap.NewNop())
	if err := handler(context.Background(), "", validSellerDeletedEvent(100, 7487768729113814280)); !errors.Is(err, dbErr) {
		t.Fatalf("expected db error for requeue, got: %v", err)
	}
	// 軟刪失敗代表那些貼文還活著。此時若仍然釋放引用，就是「業務列指著、卻沒有
	// 引用」——資產會被 media GC 實體刪除，而且重投也救不回來。
	if mediaRef.calls != 0 {
		t.Fatalf("ReleaseReferences called %d times after a failed soft delete, want 0", mediaRef.calls)
	}
}

func TestSellerDeletedHandler_CommentLikeRepoError(t *testing.T) {
	dbErr := errors.New("db error on comment like cleanup")
	post := &mockPostRepoForSeller{
		softDeleteBySellerIDFn:  func(context.Context, int64) error { return nil },
		listPostIDsBySellerIDFn: func(context.Context, int64) ([]int64, error) { return []int64{1}, nil },
	}
	comment := &mockCommentRepoForSeller{deleteLikesByUserIDFn: func(context.Context, int16, int64) error { return dbErr }}
	handler := newSellerDeletedHandler(post, comment, &fakeMediaRefClient{}, zap.NewNop())
	if err := handler(context.Background(), "", validSellerDeletedEvent(100, 7487768729113814280)); !errors.Is(err, dbErr) {
		t.Fatalf("expected db error for requeue, got: %v", err)
	}
}

// TestSellerDeletedHandler_NoLocalPayloadStructRedeclaration guards against
// reintroducing a locally-redeclared payload struct that can silently drift
// from pkg/mq's canonical wire shape. seller_deleted.go must decode directly
// into mq.SellerDeletedEvent (not mq.SellerSuspendedEvent — that type's
// AccountID/StoreID fields are ,string-tagged for the unrelated
// SELLER_SUSPENDED event and would break unmarshal here).
func TestSellerDeletedHandler_NoLocalPayloadStructRedeclaration(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "seller_deleted.go", nil, 0)
	if err != nil {
		t.Fatalf("parse seller_deleted.go: %v", err)
	}
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			if typeSpec, ok := spec.(*ast.TypeSpec); ok && typeSpec.Name.Name == "sellerDeletedPayload" {
				t.Error("seller_deleted.go must not locally redeclare sellerDeletedPayload — decode directly into mq.SellerDeletedEvent (pkg/mq/events.go:598)")
			}
		}
	}
}

// 帳號終止是「軟刪列刻意保留引用」這條規則的唯一例外，而執行它的只有這個 handler：
// reconciler 的候選集合排除軟刪列，所以沒有任何背景工作會替它補做。批次必須帶上該店
// 全部貼文 id、keep 為空——keep 非空就變成「換圖收斂」語意，該留的資產反而被留住。
//
// 順序也一起釘住：先軟刪再釋放。反過來會留下「業務列還活著、引用卻已被釋放」的窗口，
// 那正是資產被 media GC 實體刪除的形狀。
func TestSellerDeletedHandler_ReleasesMediaReferences(t *testing.T) {
	post, comment := okSellerRepos([]int64{11, 22, 33})
	seq := 0
	softDeleteOrder := 0
	post.softDeleteBySellerIDFn = func(context.Context, int64) error {
		seq++
		softDeleteOrder = seq
		return nil
	}
	mediaRef := &fakeMediaRefClient{seq: &seq}

	handler := newSellerDeletedHandler(post, comment, mediaRef, zap.NewNop())
	if err := handler(context.Background(), "", validSellerDeletedEvent(100, 7487768729113814280)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mediaRef.calls != 1 {
		t.Fatalf("ReleaseReferences called %d times, want 1", mediaRef.calls)
	}
	if mediaRef.gotRefType != media.RefTypePost {
		t.Errorf("refType = %q, want %q", mediaRef.gotRefType, media.RefTypePost)
	}
	want := []string{"11", "22", "33"}
	if !reflect.DeepEqual(mediaRef.gotRefIDs, want) {
		t.Errorf("refIDs = %v, want %v", mediaRef.gotRefIDs, want)
	}
	if len(mediaRef.gotKeep) != 0 {
		t.Errorf("keep = %v, want empty (帳號刪除語意＝釋放全部)", mediaRef.gotKeep)
	}
	if softDeleteOrder == 0 || mediaRef.releaseOrder <= softDeleteOrder {
		t.Errorf("release must happen after the soft delete, got softDelete=%d release=%d",
			softDeleteOrder, mediaRef.releaseOrder)
	}
}

// 釋放失敗必須 return err 交給 MQ retry／DLQ，不能吞掉：吞掉的話該店貼文的引用永遠
// 留在 media 上，而軟刪列不進 reconciler 的候選集合 ⇒ 沒有第二條路徑會補做。
func TestSellerDeletedHandler_ReleaseErrorPropagates(t *testing.T) {
	post, comment := okSellerRepos([]int64{11})
	rpcErr := errors.New("media unavailable")
	mediaRef := &fakeMediaRefClient{releaseErr: rpcErr}

	handler := newSellerDeletedHandler(post, comment, mediaRef, zap.NewNop())
	if err := handler(context.Background(), "", validSellerDeletedEvent(100, 7487768729113814280)); !errors.Is(err, rpcErr) {
		t.Fatalf("expected the release error for requeue, got: %v", err)
	}
}

// 沒有貼文的賣家：handler 直接跳過釋放這一步。
//
// 這個 guard 不是為了正確性——pkg/media.ReleaseReferences 對 len(refIDs) == 0 已經
// 直接 return nil（pkg/media/client.go），空集合送出去也不會出事。它省的是一次
// 白跑的 gRPC 往返（帳號終止事件對沒貼過文的賣家並不罕見）。所以不需要、也不要為
// 「media 端收到空 ref_ids 會怎樣」再補防禦性驗證，那件事已經有答案。
func TestSellerDeletedHandler_NoPostsSkipsRelease(t *testing.T) {
	post, comment := okSellerRepos(nil)
	mediaRef := &fakeMediaRefClient{}

	handler := newSellerDeletedHandler(post, comment, mediaRef, zap.NewNop())
	if err := handler(context.Background(), "", validSellerDeletedEvent(100, 7487768729113814280)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mediaRef.calls != 0 {
		t.Fatalf("ReleaseReferences called %d times, want 0 for a seller with no posts", mediaRef.calls)
	}
}

// ListPostIDsBySellerID 是 handler 的第一步，失敗必須整支中止並 return err：後面
// 三步各自都有這樣一條測試，只有這一步先前沒有。這一步失敗時特別不能繼續——它拿不到
// id 集合，硬走下去會軟刪整店貼文卻**永遠沒有任何一列被釋放**（軟刪列不在 reconciler
// 的候選集合內，沒有第二條路徑會補做），而且重投時 handler 也不會知道少做了什麼。
func TestSellerDeletedHandler_ListPostIDsError(t *testing.T) {
	dbErr := errors.New("db error on listing post ids")
	post := &mockPostRepoForSeller{
		softDeleteBySellerIDFn:  func(context.Context, int64) error { t.Fatal("should not be called"); return nil },
		listPostIDsBySellerIDFn: func(context.Context, int64) ([]int64, error) { return nil, dbErr },
	}
	comment := &mockCommentRepoForSeller{
		deleteLikesByUserIDFn: func(context.Context, int16, int64) error { t.Fatal("should not be called"); return nil },
	}
	mediaRef := &fakeMediaRefClient{}

	handler := newSellerDeletedHandler(post, comment, mediaRef, zap.NewNop())
	if err := handler(context.Background(), "", validSellerDeletedEvent(100, 7487768729113814280)); !errors.Is(err, dbErr) {
		t.Fatalf("expected db error for requeue, got: %v", err)
	}
	if mediaRef.calls != 0 {
		t.Fatalf("ReleaseReferences called %d times after the id listing failed, want 0", mediaRef.calls)
	}
}

// 大店必須分批送。pkg/media.ReleaseReferences 的 5s deadline 是**每次呼叫固定**、
// 不隨筆數放大，一次送幾千筆等於保證逾時；而這支 handler 是軟刪列釋放引用的唯一
// 路徑，逾時重試進 DLQ 之後沒有任何背景工作會補做。
//
// 斷言切法本身而不只是「呼叫超過一次」：批次上限被悄悄調大回「一次送完」時，
// 只數次數的測試會照樣綠。
func TestSellerDeletedHandler_ChunksLargeReleaseBatch(t *testing.T) {
	const posts = releaseRefBatchSize*2 + 3
	ids := make([]int64, 0, posts)
	for i := 1; i <= posts; i++ {
		ids = append(ids, int64(i))
	}
	post, comment := okSellerRepos(ids)
	mediaRef := &fakeMediaRefClient{}

	handler := newSellerDeletedHandler(post, comment, mediaRef, zap.NewNop())
	if err := handler(context.Background(), "", validSellerDeletedEvent(100, 7487768729113814280)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mediaRef.gotBatches) != 3 {
		t.Fatalf("ReleaseReferences called %d times, want 3 (%d ids / batch %d)",
			len(mediaRef.gotBatches), posts, releaseRefBatchSize)
	}
	wantSizes := []int{releaseRefBatchSize, releaseRefBatchSize, 3}
	var flat []string
	for i, batch := range mediaRef.gotBatches {
		if len(batch) != wantSizes[i] {
			t.Errorf("batch %d size = %d, want %d", i, len(batch), wantSizes[i])
		}
		if len(batch) > releaseRefBatchSize {
			t.Errorf("batch %d exceeds releaseRefBatchSize (%d > %d)", i, len(batch), releaseRefBatchSize)
		}
		flat = append(flat, batch...)
	}
	// 分批不得漏列也不得重複：這些是該店僅有的一次釋放機會。
	if len(flat) != posts {
		t.Fatalf("批次總數 = %d，want %d（分批漏了或重複送）", len(flat), posts)
	}
	for i, got := range flat {
		if want := strconv.FormatInt(int64(i+1), 10); got != want {
			t.Fatalf("扁平化後第 %d 筆 = %q, want %q（順序或內容被分批改變）", i, got, want)
		}
	}
	// 逐批斷言而不是只看 mediaRef.gotKeep：後者被每次呼叫覆寫，只驗得到最後一批。
	for i, keep := range mediaRef.gotKeeps {
		if len(keep) != 0 {
			t.Errorf("batch %d keep = %v, want empty（每一批都必須是釋放全部語意）", i, keep)
		}
	}
}

// 分批之後「某一批失敗」仍然整支 return err。冪等性讓重投重跑安全：
// ListPostIDsBySellerID 的述詞不看 deleted_at，重投導出同一個集合，已釋放過的批次
// 再送一次是 no-op——所以 handler 不需要（也刻意不）記錄批次進度。
func TestSellerDeletedHandler_ChunkedReleaseErrorStopsAtFailingBatch(t *testing.T) {
	const posts = releaseRefBatchSize*2 + 1
	ids := make([]int64, 0, posts)
	for i := 1; i <= posts; i++ {
		ids = append(ids, int64(i))
	}
	post, comment := okSellerRepos(ids)
	rpcErr := errors.New("media unavailable")
	mediaRef := &fakeMediaRefClient{failOnCall: 2, releaseErr: rpcErr}

	handler := newSellerDeletedHandler(post, comment, mediaRef, zap.NewNop())
	if err := handler(context.Background(), "", validSellerDeletedEvent(100, 7487768729113814280)); !errors.Is(err, rpcErr) {
		t.Fatalf("expected the release error for requeue, got: %v", err)
	}
	if len(mediaRef.gotBatches) != 2 {
		t.Fatalf("ReleaseReferences called %d times, want 2（第 2 批失敗即中止，不續送第 3 批）",
			len(mediaRef.gotBatches))
	}
}
