// Package resolver 把買家的品項用詞對到平台分類樹的節點（A9-D7）：分類樹加人工同義詞，沒有農產別名層。
//
// 買家的用詞不會被覆寫（AD3）：先以買家的原詞比對分類名稱；原詞有命中就只用它，同義詞不被查閱。原詞完全沒有命中時，
// 才查人工同義詞，以同義詞指向的名稱比對。Resolution.BuyerTerm 永遠是買家的原詞（去頭尾空白），不論怎麼命中。
// 同義詞放在本目錄的 synonyms.json（go:embed），空表起步，不沿用農產內容（A9-D19 S8）。
//
// 分類樹由 TaxonomyReader 提供（catalog 搬進單體前是 internal/legacy 的 client，經 cmd/hawkr 轉接），經單一 in-process 快取：TTL 過期後仍回舊樹並在背景更新，失敗退避（B8；catalog 搬進單體時整段刪除）。
package resolver

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Node 是分類樹的一個節點；ParentID 是 0 表示根。
type Node struct {
	ID       int64
	ParentID int64
	Name     string
}

// TaxonomyReader 讀平台的啟用分類樹（扁平節點）。
type TaxonomyReader interface {
	CategoryTree(ctx context.Context) ([]Node, error)
}

// Via 是命中的方式。
type Via string

const (
	// ViaExact 是買家的原詞直接命中分類名稱。
	ViaExact Via = "EXACT"
	// ViaSynonym 是原詞沒有命中、經人工同義詞命中。
	ViaSynonym Via = "SYNONYM"
)

// Match 是命中的一個分類。
type Match struct {
	CategoryID int64
	Name       string
	// Path 是從根到這個節點的名稱，以「 > 」串接。
	Path string
	Via  Via
	// RootID、RootName 是路徑最上層（大類）的節點；根節點自己的 RootID 是自己的 ID。
	RootID   int64
	RootName string
	// Leaf 是這個節點沒有子節點（末層品項）。
	Leaf bool
}

// Resolution 是一次解析的結果。
type Resolution struct {
	// BuyerTerm 是買家的原詞（去頭尾空白），不被同義詞覆寫。
	BuyerTerm string
	Matches   []Match
}

const (
	// TreeTTL 是分類樹快取的存活時間。
	TreeTTL = 30 * time.Second
	// LoadTimeout 是一次讀分類樹的上限。
	LoadTimeout = 30 * time.Second
	// RetryBackoff 是讀取失敗後不再發起讀取的時間。
	RetryBackoff = 10 * time.Second
)

//go:embed synonyms.json
var embeddedSynonyms []byte

// Synonyms 是正規化後的詞到分類名稱清單的對照。
type Synonyms map[string][]string

type synonymFile struct {
	Synonyms map[string][]string `json:"synonyms"`
}

// LoadSynonyms 解析並驗證同義詞檔：詞與目標都非空、詞已經是正規化形式、不指向自己。
func LoadSynonyms(data []byte) (Synonyms, error) {
	var f synonymFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("resolver：解 synonyms.json：%w", err)
	}
	out := make(Synonyms, len(f.Synonyms))
	for term, targets := range f.Synonyms {
		if term == "" || normalize(term) != term {
			return nil, fmt.Errorf("resolver：同義詞 %q 必須是正規化形式且非空", term)
		}
		if len(targets) == 0 {
			return nil, fmt.Errorf("resolver：同義詞 %q 沒有目標", term)
		}
		for _, t := range targets {
			if strings.TrimSpace(t) == "" {
				return nil, fmt.Errorf("resolver：同義詞 %q 有空的目標", term)
			}
			if normalize(t) == term {
				return nil, fmt.Errorf("resolver：同義詞 %q 指向自己", term)
			}
		}
		out[term] = targets
	}
	return out, nil
}

// EmbeddedSynonyms 回傳內嵌的人工同義詞；檔案不合法是程式錯誤。
func EmbeddedSynonyms() Synonyms {
	s, err := LoadSynonyms(embeddedSynonyms)
	if err != nil {
		panic(err)
	}
	return s
}

// Resolver 解析買家用詞。
type Resolver struct {
	reader   TaxonomyReader
	synonyms Synonyms
	now      func() time.Time

	mu       sync.Mutex
	tree     *index
	loading  *treeLoad
	loadedAt time.Time
	// retryAfter 之前不發起新的讀取；lastErr 是造成它的錯誤。
	retryAfter time.Time
	lastErr    error

	// beforeWait 只給測試：在呼叫者開始等待共用讀取之前呼叫。
	beforeWait func()
}

// New 建立解析器；now 是模組注入的時鐘（快取的 TTL 用它判斷）。
func New(reader TaxonomyReader, syn Synonyms, now func() time.Time) *Resolver {
	return &Resolver{reader: reader, synonyms: syn, now: now}
}

// ErrEmptyTerm 是買家的用詞去掉空白後是空的。
var ErrEmptyTerm = errors.New("resolver：用詞是空的")

// Resolve 解析一個用詞。沒有命中不是錯誤（Matches 為空）；讀不到分類樹回錯誤。
func (r *Resolver) Resolve(ctx context.Context, term string) (Resolution, error) {
	buyerTerm := strings.TrimSpace(term)
	if buyerTerm == "" {
		return Resolution{}, ErrEmptyTerm
	}
	idx, err := r.index(ctx)
	if err != nil {
		return Resolution{}, err
	}
	res := Resolution{BuyerTerm: buyerTerm}
	if m := idx.byName(normalize(buyerTerm), ViaExact); len(m) > 0 {
		res.Matches = m
		return res, nil
	}
	for _, target := range r.synonyms[normalize(buyerTerm)] {
		res.Matches = append(res.Matches, idx.byName(normalize(target), ViaSynonym)...)
	}
	return res, nil
}

// Leaves 回傳大類 rootID 底下的末層節點（不含大類自己），順序依分類樹讀取結果。
func (r *Resolver) Leaves(ctx context.Context, rootID int64) ([]Match, error) {
	idx, err := r.index(ctx)
	if err != nil {
		return nil, err
	}
	var out []Match
	for _, id := range idx.order {
		n := idx.nodes[id]
		if n.ID == rootID || idx.hasChild[n.ID] {
			continue
		}
		if m := idx.match(n, ViaExact); m.RootID == rootID {
			out = append(out, m)
		}
	}
	return out, nil
}

// treeLoad 是一次進行中的分類樹讀取；所有等待者共用它的結果。
type treeLoad struct {
	done chan struct{}
	err  error
}

// index 回傳可用的分類樹（stale-while-revalidate）：
//
//   - 有快取樹：立即回它，不等讀取；過期時在背景更新（同時只有一次讀取），失敗後 RetryBackoff 內不再發起讀取，期間繼續用舊樹。
//   - 沒有快取樹：所有呼叫共用同一次讀取，等待者可因 ctx 結束而離開（讀取本身不隨呼叫者取消）；
//     失敗後 RetryBackoff 內直接回上一次的錯誤，不再發起讀取。
func (r *Resolver) index(ctx context.Context) (*index, error) {
	r.mu.Lock()
	now := r.now()
	if r.tree != nil {
		idx := r.tree
		if now.Sub(r.loadedAt) >= TreeTTL && r.loading == nil && !now.Before(r.retryAfter) {
			r.startLoadLocked(ctx)
		}
		r.mu.Unlock()
		return idx, nil
	}
	l := r.loading
	if l == nil {
		if now.Before(r.retryAfter) {
			err := r.lastErr
			r.mu.Unlock()
			return nil, fmt.Errorf("resolver：讀分類樹：%w", err)
		}
		l = r.startLoadLocked(ctx)
	}
	r.mu.Unlock()

	if r.beforeWait != nil {
		r.beforeWait()
	}
	select {
	case <-l.done:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	r.mu.Lock()
	idx := r.tree
	r.mu.Unlock()
	if l.err != nil {
		return nil, fmt.Errorf("resolver：讀分類樹：%w", l.err)
	}
	return idx, nil
}

func (r *Resolver) startLoadLocked(ctx context.Context) *treeLoad {
	l := &treeLoad{done: make(chan struct{})}
	r.loading = l
	go r.load(context.WithoutCancel(ctx), l)
	return l
}

// load 讀一次分類樹。reader 的 panic 轉成錯誤：這是背景 goroutine，沒人接就會讓整個進程結束。
func (r *Resolver) load(ctx context.Context, l *treeLoad) {
	var idx *index
	err := func() (err error) {
		defer func() {
			if p := recover(); p != nil {
				err = fmt.Errorf("讀取時 panic：%v", p)
			}
		}()
		ctx, cancel := context.WithTimeout(ctx, LoadTimeout)
		defer cancel()
		nodes, err := r.reader.CategoryTree(ctx)
		if err != nil {
			return err
		}
		idx = buildIndex(nodes)
		return nil
	}()
	r.mu.Lock()
	now := r.now()
	if err == nil {
		r.tree = idx
		r.loadedAt = now
		r.retryAfter = time.Time{}
		r.lastErr = nil
	} else {
		r.retryAfter = now.Add(RetryBackoff)
		r.lastErr = err
	}
	r.loading = nil
	l.err = err
	r.mu.Unlock()
	close(l.done)
}

type index struct {
	nodes  map[int64]Node
	byNorm map[string][]int64
	// order 是節點在分類樹讀取結果中的順序；hasChild 記錄有子節點的節點。
	order    []int64
	hasChild map[int64]bool
}

func buildIndex(nodes []Node) *index {
	idx := &index{nodes: make(map[int64]Node, len(nodes)), byNorm: map[string][]int64{}, hasChild: map[int64]bool{}}
	for _, n := range nodes {
		idx.nodes[n.ID] = n
		idx.order = append(idx.order, n.ID)
	}
	for _, n := range nodes {
		if n.ParentID != 0 {
			idx.hasChild[n.ParentID] = true
		}
		k := normalize(n.Name)
		idx.byNorm[k] = append(idx.byNorm[k], n.ID)
	}
	return idx
}

func (x *index) byName(key string, via Via) []Match {
	var out []Match
	for _, id := range x.byNorm[key] {
		out = append(out, x.match(x.nodes[id], via))
	}
	return out
}

func (x *index) match(n Node, via Via) Match {
	root := x.root(n)
	return Match{CategoryID: n.ID, Name: n.Name, Path: x.path(n), Via: via, RootID: root.ID, RootName: root.Name, Leaf: !x.hasChild[n.ID]}
}

func (x *index) root(n Node) Node {
	seen := map[int64]bool{n.ID: true}
	for n.ParentID != 0 && !seen[n.ParentID] {
		parent, ok := x.nodes[n.ParentID]
		if !ok {
			break
		}
		seen[parent.ID] = true
		n = parent
	}
	return n
}

func (x *index) path(n Node) string {
	parts := []string{n.Name}
	seen := map[int64]bool{n.ID: true}
	for p := n.ParentID; p != 0 && !seen[p]; {
		parent, ok := x.nodes[p]
		if !ok {
			break
		}
		seen[p] = true
		parts = append([]string{parent.Name}, parts...)
		p = parent.ParentID
	}
	return strings.Join(parts, " > ")
}

// normalize 是比對用的形式：NFKC、小寫、去掉所有空白。
func normalize(s string) string {
	s = norm.NFKC.String(s)
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return unicode.ToLower(r)
	}, s)
}
