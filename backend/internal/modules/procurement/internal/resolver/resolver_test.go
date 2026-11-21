package resolver

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"
)

type fakeReader struct {
	nodes []Node
	err   error
	calls int
}

func (f *fakeReader) CategoryTree(context.Context) ([]Node, error) {
	f.calls++
	return f.nodes, f.err
}

var tree = []Node{
	{ID: 1, Name: "服飾"},
	{ID: 2, ParentID: 1, Name: "外套"},
	{ID: 3, ParentID: 1, Name: "夾克"},
	{ID: 4, Name: "3C"},
	{ID: 5, ParentID: 4, Name: "耳機"},
}

func newResolver(r TaxonomyReader, syn Synonyms) *Resolver {
	return New(r, syn, func() time.Time { return time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC) })
}

// AD3：買家的原詞有命中分類時，同義詞不被查閱，也不覆寫買家的詞。
func TestSynonymNeverOverridesTheBuyersOwnTerm(t *testing.T) {
	syn := Synonyms{"外套": {"夾克"}} // 同義詞把「外套」指向「夾克」
	got, err := newResolver(&fakeReader{nodes: tree}, syn).Resolve(context.Background(), " 外套 ")
	if err != nil {
		t.Fatal(err)
	}
	if got.BuyerTerm != "外套" {
		t.Fatalf("BuyerTerm = %q，買家的原詞不能被覆寫", got.BuyerTerm)
	}
	if len(got.Matches) != 1 || got.Matches[0].CategoryID != 2 || got.Matches[0].Via != ViaExact {
		t.Fatalf("原詞「外套」有命中分類 2，同義詞不該被用：%+v", got.Matches)
	}
}

// 原詞沒有命中時才查同義詞；BuyerTerm 仍是原詞。
func TestSynonymIsUsedOnlyWhenTheBuyersTermMatchesNothing(t *testing.T) {
	syn := Synonyms{"耳麥": {"耳機"}}
	got, err := newResolver(&fakeReader{nodes: tree}, syn).Resolve(context.Background(), "耳麥")
	if err != nil {
		t.Fatal(err)
	}
	if got.BuyerTerm != "耳麥" {
		t.Fatalf("BuyerTerm = %q", got.BuyerTerm)
	}
	if len(got.Matches) != 1 || got.Matches[0].CategoryID != 5 || got.Matches[0].Via != ViaSynonym || got.Matches[0].Path != "3C > 耳機" {
		t.Fatalf("matches = %+v", got.Matches)
	}
}

func TestNoMatchIsNotAnErrorAndSpaceCaseAreNormalized(t *testing.T) {
	r := newResolver(&fakeReader{nodes: []Node{{ID: 9, Name: "USB 線"}}}, Synonyms{})
	got, err := r.Resolve(context.Background(), "沒有的東西")
	if err != nil || len(got.Matches) != 0 || got.BuyerTerm != "沒有的東西" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	got, err = r.Resolve(context.Background(), "usb線")
	if err != nil || len(got.Matches) != 1 || got.Matches[0].CategoryID != 9 {
		t.Fatalf("正規化（大小寫、空白）後應命中：%+v err=%v", got, err)
	}
}

func TestEmptyTermAndReaderFailure(t *testing.T) {
	if _, err := newResolver(&fakeReader{nodes: tree}, nil).Resolve(context.Background(), "  "); !errors.Is(err, ErrEmptyTerm) {
		t.Fatalf("err = %v", err)
	}
	boom := errors.New("boom")
	if _, err := newResolver(&fakeReader{err: boom}, nil).Resolve(context.Background(), "外套"); !errors.Is(err, boom) {
		t.Fatalf("讀不到分類樹要回錯誤：%v", err)
	}
}

// 分類樹經 TTL 快取：TTL 內只讀一次，過期後重讀。
func TestTreeIsCachedForTheTTL(t *testing.T) {
	reader := &fakeReader{nodes: tree}
	now := time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)
	r := New(reader, nil, func() time.Time { return now })
	for range 3 {
		if _, err := r.Resolve(context.Background(), "外套"); err != nil {
			t.Fatal(err)
		}
	}
	if reader.calls != 1 {
		t.Fatalf("TTL 內讀了 %d 次分類樹，要 1 次", reader.calls)
	}
	now = now.Add(TreeTTL + time.Second)
	if _, err := r.Resolve(context.Background(), "外套"); err != nil {
		t.Fatal(err)
	}
	if reader.calls != 2 {
		t.Fatalf("TTL 過期後讀了 %d 次，要 2 次", reader.calls)
	}
}

func TestLoadSynonymsRejectsBadEntries(t *testing.T) {
	for name, in := range map[string]string{
		"空目標":     `{"synonyms":{"a":[]}}`,
		"指向自己":    `{"synonyms":{"耳機":["耳機"]}}`,
		"詞未正規化":   `{"synonyms":{"USB 線":["x"]}}`,
		"目標是空白":   `{"synonyms":{"a":[" "]}}`,
		"不是 JSON": `nope`,
	} {
		if _, err := LoadSynonyms([]byte(in)); err == nil {
			t.Errorf("%s：應該被拒絕", name)
		}
	}
}

// 內嵌的人工同義詞檔合法，而且空表起步（A9-D19 S8：不沿用農產內容）。
func TestEmbeddedSynonymsAreValidAndStartEmpty(t *testing.T) {
	if got := EmbeddedSynonyms(); len(got) != 0 {
		t.Fatalf("同義詞表應空表起步，現有 %d 筆", len(got))
	}
}

// gateReader 讓測試用 channel 控制讀取何時完成。
type gateReader struct {
	mu      sync.Mutex
	calls   int
	started chan struct{}
	release chan struct{}
	nodes   []Node
	err     error
}

func (g *gateReader) CategoryTree(context.Context) ([]Node, error) {
	g.mu.Lock()
	g.calls++
	g.mu.Unlock()
	g.started <- struct{}{}
	<-g.release
	return g.nodes, g.err
}

func (g *gateReader) count() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.calls
}

// 慢的分類服務下，N 個並行解析只讀一次。
func TestConcurrentResolvesShareOneLoad(t *testing.T) {
	g := &gateReader{started: make(chan struct{}, 8), release: make(chan struct{}), nodes: tree}
	r := newResolver(g, nil)
	const n = 8
	errs := make(chan error, n)
	for range n {
		go func() { _, err := r.Resolve(context.Background(), "外套"); errs <- err }()
	}
	<-g.started
	// 等其餘呼叫都進入等待：它們若各自讀取，started 會再收到訊號。
	for range 1000 {
		runtime.Gosched()
	}
	close(g.release)
	for range n {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if c := g.count(); c != 1 {
		t.Fatalf("%d 個並行解析讀了 %d 次分類樹，要 1 次", n, c)
	}
}

// 等待者的 ctx 結束就立即離開，不等讀取完成。
func TestCancelledContextReturnsWithoutWaitingForTheLoad(t *testing.T) {
	g := &gateReader{started: make(chan struct{}, 2), release: make(chan struct{}), nodes: tree}
	r := newResolver(g, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := r.Resolve(ctx, "外套"); done <- err }()
	<-g.started
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v，要 context.Canceled", err)
	}
	close(g.release)
}

// 重讀失敗時繼續用舊樹。
func TestFailedReloadKeepsServingTheOldTree(t *testing.T) {
	reader := &fakeReader{nodes: tree}
	now := time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)
	r := New(reader, nil, func() time.Time { return now })
	if _, err := r.Resolve(context.Background(), "外套"); err != nil {
		t.Fatal(err)
	}
	reader.err = errors.New("boom")
	now = now.Add(TreeTTL + time.Second)
	res, err := r.Resolve(context.Background(), "外套")
	if err != nil || len(res.Matches) == 0 {
		t.Fatalf("重讀失敗要回舊樹的結果：%v %v", res, err)
	}
}
