package search

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/candidate"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/resolver"
)

type stubLeaf struct {
	leaf, fresh       bool
	leafErr, freshErr error
	onLeaf            func()
	freshCalls        int
}

func (s *stubLeaf) Leaf(context.Context, int64) (resolver.Match, bool, error) {
	if s.onLeaf != nil {
		s.onLeaf()
	}
	return resolver.Match{}, s.leaf, s.leafErr
}

func (s *stubLeaf) LeafFresh(context.Context, int64) (resolver.Match, bool, error) {
	s.freshCalls++
	return resolver.Match{}, s.fresh, s.freshErr
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

func TestFindSearchesWhenTheCachedTreeSaysNotLeafButTheFreshTreeStillHasIt(t *testing.T) {
	l, s := &stubLeaf{leaf: false, fresh: true}, &stubSearch{}
	f, err := executorOf(l, s).find(context.Background(), 1, byCategory)
	if err != nil || f.categoryGone || len(f.hits) != 1 || s.calls != 1 || l.freshCalls != 1 {
		t.Fatalf("found=%+v err=%v searches=%d fresh=%d", f, err, s.calls, l.freshCalls)
	}
}

func TestFindOnlyDeclaresTheCategoryGoneWhenTheFreshTreeConfirmsIt(t *testing.T) {
	l, s := &stubLeaf{leaf: false, fresh: false}, &stubSearch{}
	f, err := executorOf(l, s).find(context.Background(), 1, byCategory)
	if err != nil || !f.categoryGone || s.calls != 0 {
		t.Fatalf("found=%+v err=%v searches=%d", f, err, s.calls)
	}
}

func TestFindSkipsTheFreshCheckWhenTheCachedTreeSaysLeaf(t *testing.T) {
	l, s := &stubLeaf{leaf: true}, &stubSearch{}
	if f, err := executorOf(l, s).find(context.Background(), 1, byCategory); err != nil || f.categoryGone || l.freshCalls != 0 {
		t.Fatalf("found=%+v err=%v fresh=%d", f, err, l.freshCalls)
	}
}

func TestFindReturnsAnErrorNotCategoryGoneWhenTheTreeCannotBeRead(t *testing.T) {
	boom := errors.New("catalog down")
	for name, l := range map[string]*stubLeaf{
		"leaf read fails": {leafErr: boom},
		"no fresh tree":   {leaf: false, freshErr: boom},
	} {
		s := &stubSearch{}
		f, err := executorOf(l, s).find(context.Background(), 1, byCategory)
		if !errors.Is(err, boom) || f.categoryGone || s.calls != 0 {
			t.Errorf("%s：found=%+v err=%v searches=%d", name, f, err, s.calls)
		}
	}
}

func TestFindReturnsTheContextErrorWhenCancelledDuringTheSearch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &stubSearch{err: errors.New("rpc aborted"), onGet: cancel}
	if _, err := executorOf(&stubLeaf{leaf: true}, s).find(ctx, 1, byCategory); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v，要 context.Canceled", err)
	}
}

func TestFindReturnsTheContextErrorWhenCancelledAfterASuccessfulSearch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &stubSearch{onGet: cancel}
	if _, err := executorOf(&stubLeaf{leaf: true}, s).find(ctx, 1, byCategory); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v，搜尋成功但 ctx 已結束要回 context.Canceled，不進 commit", err)
	}
}

func TestFindReturnsTheContextErrorWhenCancelledDuringTheCategoryCheck(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	l := &stubLeaf{leafErr: errors.New("rpc aborted"), onLeaf: cancel}
	if _, err := executorOf(l, &stubSearch{}).find(ctx, 1, byCategory); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v，要 context.Canceled", err)
	}
}
