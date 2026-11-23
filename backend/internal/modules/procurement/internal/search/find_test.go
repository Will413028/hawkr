package search

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/candidate"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/resolver"
)

type stubLeaf struct {
	leaf       bool
	err        error
	onLeaf     func()
	calls      int
	notBefores []time.Time
}

func (s *stubLeaf) LeafAsOf(_ context.Context, _ int64, notBefore time.Time) (resolver.Match, bool, error) {
	s.calls++
	s.notBefores = append(s.notBefores, notBefore)
	if s.onLeaf != nil {
		s.onLeaf()
	}
	return resolver.Match{}, s.leaf, s.err
}

type stubSearch struct {
	calls int
	err   error
	onGet func()
}

func (s *stubSearch) Page(context.Context, candidate.Query, int) (candidate.Page, error) {
	s.calls++
	if s.onGet != nil {
		s.onGet()
	}
	return candidate.Page{Hits: []candidate.Hit{{ProductID: 1, StoreID: 7}}, Last: true}, s.err
}

type allOpen struct{}

func (allOpen) OpenStoreIDs(_ context.Context, ids []int64) (map[int64]struct{}, error) {
	out := map[int64]struct{}{}
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out, nil
}

type noBlocks struct{}

func (noBlocks) StoreIDsBlockingBuyer(context.Context, int64, []int64) (map[int64]struct{}, error) {
	return nil, nil
}

func executorOf(l *stubLeaf, s *stubSearch) Executor {
	return Executor{Searcher: s, Open: allOpen{}, Blocks: noBlocks{}, Resolver: l}
}

var byCategory = candidate.Query{CategoryID: 18}

var asOf = time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)

func TestFindSearchesWhenTheTreeConfirmsALeafAndPassesTheRevisionTimeAsTheLowerBound(t *testing.T) {
	l, s := &stubLeaf{leaf: true}, &stubSearch{}
	f, err := executorOf(l, s).find(context.Background(), 1, byCategory, asOf)
	if err != nil || f.categoryGone || len(f.hits) != 1 || s.calls != 1 || l.calls != 1 || !l.notBefores[0].Equal(asOf) {
		t.Fatalf("found=%+v err=%v searches=%d calls=%d notBefores=%v", f, err, s.calls, l.calls, l.notBefores)
	}
}

func TestFindDeclaresTheCategoryGoneWhenTheTreeSaysNotLeaf(t *testing.T) {
	l, s := &stubLeaf{leaf: false}, &stubSearch{}
	f, err := executorOf(l, s).find(context.Background(), 1, byCategory, asOf)
	if err != nil || !f.categoryGone || s.calls != 0 {
		t.Fatalf("found=%+v err=%v searches=%d", f, err, s.calls)
	}
}

func TestFindSkipsTheCategoryCheckForAKeywordQuery(t *testing.T) {
	l, s := &stubLeaf{}, &stubSearch{}
	if f, err := executorOf(l, s).find(context.Background(), 1, candidate.Query{Keyword: "蘋果"}, asOf); err != nil || f.categoryGone || l.calls != 0 {
		t.Fatalf("found=%+v err=%v calls=%d", f, err, l.calls)
	}
}

func TestFindReturnsAnErrorNotCategoryGoneWhenTheTreeCannotBeRead(t *testing.T) {
	boom := errors.New("catalog down")
	l, s := &stubLeaf{err: boom}, &stubSearch{}
	f, err := executorOf(l, s).find(context.Background(), 1, byCategory, asOf)
	if !errors.Is(err, boom) || f.categoryGone || s.calls != 0 {
		t.Fatalf("found=%+v err=%v searches=%d", f, err, s.calls)
	}
}

func TestFindReturnsTheContextErrorWhenCancelledDuringTheSearch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &stubSearch{err: errors.New("rpc aborted"), onGet: cancel}
	if _, err := executorOf(&stubLeaf{leaf: true}, s).find(ctx, 1, byCategory, asOf); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v，要 context.Canceled", err)
	}
}

func TestFindReturnsTheContextErrorWhenCancelledAfterASuccessfulSearch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &stubSearch{onGet: cancel}
	if _, err := executorOf(&stubLeaf{leaf: true}, s).find(ctx, 1, byCategory, asOf); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v，搜尋成功但 ctx 已結束要回 context.Canceled，不進 commit", err)
	}
}

func TestFindReturnsTheContextErrorWhenCancelledDuringTheCategoryCheck(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	l := &stubLeaf{err: errors.New("rpc aborted"), onLeaf: cancel}
	if _, err := executorOf(l, &stubSearch{}).find(ctx, 1, byCategory, asOf); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v，要 context.Canceled", err)
	}
}
