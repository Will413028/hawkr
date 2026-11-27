package resolver

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeReader struct {
	mu    sync.Mutex
	nodes []Node
	err   error
	calls int
}

func (f *fakeReader) CategoryTree(context.Context) ([]Node, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.nodes, f.err
}

func (f *fakeReader) set(nodes []Node, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nodes, f.err = nodes, err
}

func (f *fakeReader) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// testClock 是可以從測試推進、背景讀取也能安全讀取的時鐘；測試不依賴真實時間。
type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *testClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *testClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func newClock() *testClock { return &testClock{t: time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)} }

// waitIdle 等進行中的背景讀取結束（沒有就立即回）。
func waitIdle(r *Resolver) {
	r.mu.Lock()
	l := r.loading
	r.mu.Unlock()
	if l != nil {
		<-l.done
	}
}

var tree = []Node{
	{ID: categoryID(1), Name: "服飾"},
	{ID: categoryID(2), ParentID: parentOf(1), Name: "外套"},
	{ID: categoryID(3), ParentID: parentOf(1), Name: "夾克"},
	{ID: categoryID(4), Name: "3C"},
	{ID: categoryID(5), ParentID: parentOf(4), Name: "耳機"},
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
	if len(got.Matches) != 1 || got.Matches[0].CategoryID != categoryID(2) || got.Matches[0].Via != ViaExact {
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
	if len(got.Matches) != 1 || got.Matches[0].CategoryID != categoryID(5) || got.Matches[0].Via != ViaSynonym || got.Matches[0].Path != "3C > 耳機" {
		t.Fatalf("matches = %+v", got.Matches)
	}
}

func TestNoMatchIsNotAnErrorAndSpaceCaseAreNormalized(t *testing.T) {
	r := newResolver(&fakeReader{nodes: []Node{{ID: categoryID(9), Name: "USB 線"}}}, Synonyms{})
	got, err := r.Resolve(context.Background(), "沒有的東西")
	if err != nil || len(got.Matches) != 0 || got.BuyerTerm != "沒有的東西" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	got, err = r.Resolve(context.Background(), "usb線")
	if err != nil || len(got.Matches) != 1 || got.Matches[0].CategoryID != categoryID(9) {
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

// 分類樹經 TTL 快取：TTL 內只讀一次，過期後再讀。
func TestTreeIsCachedForTheTTL(t *testing.T) {
	reader := &fakeReader{nodes: tree}
	clk := newClock()
	r := New(reader, nil, clk.now)
	for range 3 {
		if _, err := r.Resolve(context.Background(), "外套"); err != nil {
			t.Fatal(err)
		}
	}
	if c := reader.count(); c != 1 {
		t.Fatalf("TTL 內讀了 %d 次分類樹，要 1 次", c)
	}
	clk.advance(TreeTTL + time.Second)
	if _, err := r.Resolve(context.Background(), "外套"); err != nil {
		t.Fatal(err)
	}
	waitIdle(r)
	if c := reader.count(); c != 2 {
		t.Fatalf("TTL 過期後讀了 %d 次，要 2 次", c)
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
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.nodes, g.err
}

func (g *gateReader) count() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.calls
}

// 慢的分類服務下，N 個並行解析只讀一次：先確定 N 個呼叫都已經等在共用的讀取上，才放行並斷言讀取次數。
func TestConcurrentResolvesShareOneLoad(t *testing.T) {
	g := &gateReader{started: make(chan struct{}, 16), release: make(chan struct{}), nodes: tree}
	r := newResolver(g, nil)
	const n = 8
	waiting := make(chan struct{}, n)
	r.beforeWait = func() { waiting <- struct{}{} }
	errs := make(chan error, n)
	for range n {
		go func() { _, err := r.Resolve(context.Background(), "外套"); errs <- err }()
	}
	for range n {
		<-waiting
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
	waitIdle(r)
}

// R2-4：TTL 過期後有舊樹就立即回舊樹，不等重讀；重讀在背景完成後換成新樹，同時只有一次讀取。
func TestStaleTreeIsServedImmediatelyWhileReloadingInTheBackground(t *testing.T) {
	g := &gateReader{started: make(chan struct{}, 4), release: make(chan struct{}, 4), nodes: tree}
	clk := newClock()
	r := New(g, nil, clk.now)
	g.release <- struct{}{}
	if _, err := r.Resolve(context.Background(), "外套"); err != nil {
		t.Fatal(err)
	}
	<-g.started

	g.mu.Lock()
	g.nodes = []Node{{ID: categoryID(7), Name: "外套"}}
	g.mu.Unlock()
	clk.advance(TreeTTL + time.Second)
	for range 3 {
		res, err := r.Resolve(context.Background(), "外套") // 重讀被擋住；這些呼叫不能等它
		if err != nil || len(res.Matches) != 1 || res.Matches[0].CategoryID != categoryID(2) {
			t.Fatalf("過期後重讀還沒完成，要立即回舊樹：%+v %v", res, err)
		}
	}
	<-g.started
	g.release <- struct{}{}
	waitIdle(r)
	if c := g.count(); c != 2 {
		t.Fatalf("過期後的多個呼叫讀了 %d 次分類樹（含第一次），要 2 次", c)
	}
	res, err := r.Resolve(context.Background(), "外套")
	if err != nil || len(res.Matches) != 1 || res.Matches[0].CategoryID != categoryID(7) {
		t.Fatalf("背景重讀完成後要換成新樹：%+v %v", res, err)
	}
}

// 重讀失敗時繼續用舊樹。
func TestFailedReloadKeepsServingTheOldTree(t *testing.T) {
	reader := &fakeReader{nodes: tree}
	clk := newClock()
	r := New(reader, nil, clk.now)
	if _, err := r.Resolve(context.Background(), "外套"); err != nil {
		t.Fatal(err)
	}
	reader.set(nil, errors.New("boom"))
	clk.advance(TreeTTL + time.Second)
	res, err := r.Resolve(context.Background(), "外套")
	waitIdle(r)
	if err != nil || len(res.Matches) == 0 {
		t.Fatalf("重讀失敗要回舊樹的結果：%v %v", res, err)
	}
	res, err = r.Resolve(context.Background(), "外套")
	if err != nil || len(res.Matches) == 0 {
		t.Fatalf("重讀失敗之後仍要回舊樹：%v %v", res, err)
	}
}

// R2-4：重讀失敗後 RetryBackoff 內不再發起讀取（有舊樹時）；退避過了才再試。
func TestFailedReloadBacksOffWithAnOldTree(t *testing.T) {
	reader := &fakeReader{nodes: tree}
	clk := newClock()
	r := New(reader, nil, clk.now)
	if _, err := r.Resolve(context.Background(), "外套"); err != nil {
		t.Fatal(err)
	}
	reader.set(nil, errors.New("boom"))
	clk.advance(TreeTTL + time.Second)
	for range 5 {
		if _, err := r.Resolve(context.Background(), "外套"); err != nil {
			t.Fatal(err)
		}
		waitIdle(r)
	}
	if c := reader.count(); c != 2 {
		t.Fatalf("失敗後退避期內讀了 %d 次（含第一次），要 2 次", c)
	}
	clk.advance(RetryBackoff)
	if _, err := r.Resolve(context.Background(), "外套"); err != nil {
		t.Fatal(err)
	}
	waitIdle(r)
	if c := reader.count(); c != 3 {
		t.Fatalf("退避過後要再試一次，累計 %d 次，要 3 次", c)
	}
}

// R2-4：沒有舊樹時讀取失敗，退避期內直接回上一次的錯誤，不再發起讀取。
func TestFailedFirstLoadBacksOffWithoutATree(t *testing.T) {
	boom := errors.New("boom")
	reader := &fakeReader{err: boom}
	clk := newClock()
	r := New(reader, nil, clk.now)
	for range 4 {
		if _, err := r.Resolve(context.Background(), "外套"); !errors.Is(err, boom) {
			t.Fatalf("err = %v，要 boom", err)
		}
	}
	if c := reader.count(); c != 1 {
		t.Fatalf("退避期內讀了 %d 次，要 1 次", c)
	}
	reader.set(tree, nil)
	clk.advance(RetryBackoff)
	if _, err := r.Resolve(context.Background(), "外套"); err != nil {
		t.Fatalf("退避過後要重試並成功：%v", err)
	}
}

type panicReader struct{}

func (panicReader) CategoryTree(context.Context) ([]Node, error) { panic("reader exploded") }

// R2-7：reader panic 不能殺掉進程；沒有舊樹時 Resolve 回錯誤。
func TestReaderPanicBecomesAnError(t *testing.T) {
	r := newResolver(panicReader{}, nil)
	if _, err := r.Resolve(context.Background(), "外套"); err == nil {
		t.Fatal("reader panic 時 Resolve 要回錯誤")
	}
	waitIdle(r)
}

// R2-7：有舊樹時背景重讀 panic，繼續服務舊樹。
func TestReaderPanicOnReloadKeepsServingTheOldTree(t *testing.T) {
	clk := newClock()
	var reader switchReader
	reader.set(&fakeReader{nodes: tree})
	r := New(&reader, nil, clk.now)
	if _, err := r.Resolve(context.Background(), "外套"); err != nil {
		t.Fatal(err)
	}
	reader.set(panicReader{})
	clk.advance(TreeTTL + time.Second)
	res, err := r.Resolve(context.Background(), "外套")
	waitIdle(r)
	if err != nil || len(res.Matches) == 0 {
		t.Fatalf("背景重讀 panic 時要回舊樹：%v %v", res, err)
	}
}

type switchReader struct {
	mu sync.Mutex
	r  TaxonomyReader
}

func (s *switchReader) set(r TaxonomyReader) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.r = r
}

func (s *switchReader) CategoryTree(ctx context.Context) ([]Node, error) {
	s.mu.Lock()
	r := s.r
	s.mu.Unlock()
	return r.CategoryTree(ctx)
}

func TestLeafLooksUpByCategoryIDAndOnlyLeavesQualify(t *testing.T) {
	nodes := []Node{{ID: categoryID(1), Name: "水果"}, {ID: categoryID(2), ParentID: parentOf(1), Name: "蘋果"}, {ID: categoryID(3), Name: "孤兒"}}
	r := newResolver(&fakeReader{nodes: nodes}, nil)
	for _, tc := range []struct {
		id   int
		want bool
	}{{2, true}, {1, false}, {99, false}, {3, true}} {
		m, ok, err := r.Leaf(context.Background(), categoryID(tc.id))
		if err != nil || ok != tc.want {
			t.Fatalf("Leaf(%d) = %+v %v %v，要 ok=%v", tc.id, m, ok, err, tc.want)
		}
	}
	if m, _, _ := r.Leaf(context.Background(), categoryID(2)); m.RootID != categoryID(1) || m.Name != "蘋果" {
		t.Fatalf("%+v", m)
	}
}

func TestLeafAsOfReloadsATreeLoadedBeforeNotBeforeAndSeesTheNewLeaf(t *testing.T) {
	reader := &fakeReader{nodes: tree}
	clk := newClock()
	r := New(reader, nil, clk.now)
	ctx := context.Background()
	if _, ok, err := r.LeafAsOf(ctx, categoryID(9), clk.now()); err != nil || ok {
		t.Fatalf("分類 9 還不存在：%v %v", ok, err)
	}
	reader.set(append(append([]Node{}, tree...), Node{ID: categoryID(9), ParentID: parentOf(1), Name: "背心"}), nil)
	clk.advance(time.Second)
	if _, ok, err := r.LeafAsOf(ctx, categoryID(9), clk.now()); err != nil || !ok || reader.count() != 2 {
		t.Fatalf("樹早於 notBefore 要在 TTL 之內重讀並看到新末層：ok=%v err=%v calls=%d", ok, err, reader.count())
	}
}

func TestLeafAsOfAnswersFromACacheLoadedAtOrAfterNotBeforeWithoutReloading(t *testing.T) {
	reader := &fakeReader{nodes: tree}
	clk := newClock()
	r := New(reader, nil, clk.now)
	ctx := context.Background()
	loaded := clk.now()
	if _, ok, err := r.LeafAsOf(ctx, categoryID(2), loaded); err != nil || !ok {
		t.Fatalf("%v %v", ok, err)
	}
	clk.advance(time.Second)
	if _, ok, err := r.LeafAsOf(ctx, categoryID(2), loaded); err != nil || !ok || reader.count() != 1 {
		t.Fatalf("夠新的樹不重讀：ok=%v err=%v calls=%d", ok, err, reader.count())
	}
	if _, ok, err := r.LeafAsOf(ctx, categoryID(1), loaded); err != nil || ok || reader.count() != 1 {
		t.Fatalf("夠新的樹確定不是末層：ok=%v err=%v calls=%d", ok, err, reader.count())
	}
}

func TestLeafAsOfReloadsAnExpiredTreeEvenWhenTheCachedAnswerIsPositive(t *testing.T) {
	reader := &fakeReader{nodes: tree}
	clk := newClock()
	r := New(reader, nil, clk.now)
	ctx := context.Background()
	notBefore := clk.now()
	if _, ok, err := r.LeafAsOf(ctx, categoryID(2), notBefore); err != nil || !ok {
		t.Fatalf("%v %v", ok, err)
	}
	reader.set([]Node{{ID: categoryID(1), Name: "服飾"}, {ID: categoryID(2), ParentID: parentOf(1), Name: "外套"}, {ID: categoryID(9), ParentID: parentOf(2), Name: "羽絨外套"}}, nil)
	clk.advance(TreeTTL + time.Second)
	if _, ok, err := r.LeafAsOf(ctx, categoryID(2), notBefore); err != nil || ok || reader.count() != 2 {
		t.Fatalf("過期的樹要重讀，2 已有子節點：ok=%v err=%v calls=%d", ok, err, reader.count())
	}
}

func TestLeafAsOfWithAStaleTreeAndFailedReloadReturnsAnErrorNotTheOldAnswer(t *testing.T) {
	reader := &fakeReader{nodes: tree}
	clk := newClock()
	r := New(reader, nil, clk.now)
	ctx := context.Background()
	if _, _, err := r.LeafAsOf(ctx, categoryID(2), clk.now()); err != nil {
		t.Fatal(err)
	}
	clk.advance(time.Second)
	notBefore := clk.now()
	boom := errors.New("catalog down")
	reader.set(nil, boom)
	if _, ok, err := r.LeafAsOf(ctx, categoryID(2), notBefore); !errors.Is(err, boom) || ok {
		t.Fatalf("讀取失敗要回錯誤：%v %v", ok, err)
	}
	if _, ok, err := r.LeafAsOf(ctx, categoryID(2), notBefore); err == nil || ok {
		t.Fatalf("退避期間仍要回錯誤，不用舊樹：%v %v", ok, err)
	}
	if _, ok, err := r.Leaf(ctx, categoryID(2)); err != nil || !ok {
		t.Fatalf("Leaf 仍回舊樹：%v %v", ok, err)
	}
}
