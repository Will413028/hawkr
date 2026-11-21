package candidate

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
)

type fakeOpen struct {
	open map[int64]struct{}
	err  error
	got  []int64
}

func (f *fakeOpen) OpenStoreIDs(_ context.Context, ids []int64) (map[int64]struct{}, error) {
	f.got = ids
	return f.open, f.err
}

func found(ids ...int64) []Found {
	out := make([]Found, 0, len(ids))
	for _, id := range ids {
		out = append(out, Found{LLMCandidate: model.LLMCandidate{Candidate: model.Candidate{StoreID: id}}})
	}
	return out
}

func ids(fs []Found) []int64 {
	out := []int64{}
	for _, f := range fs {
		out = append(out, f.StoreID)
	}
	return out
}

// S9：候選只收 OPEN 的店，維持輸入順序；暫停或不存在的店不會成為候選。
func TestKeepOpenDropsStoresThatAreNotOpen(t *testing.T) {
	gate := &fakeOpen{open: map[int64]struct{}{30: {}, 10: {}}}
	got, err := KeepOpen(context.Background(), gate, found(10, 20, 30))
	if err != nil {
		t.Fatal(err)
	}
	if want := []int64{10, 30}; !reflect.DeepEqual(ids(got), want) {
		t.Fatalf("候選 = %v，要 %v", ids(got), want)
	}
	if want := []int64{10, 20, 30}; !reflect.DeepEqual(gate.got, want) {
		t.Fatalf("查了 %v，要把全部都問過 %v", gate.got, want)
	}
}

// fail-closed：讀不到店家狀態時不回任何候選。
func TestKeepOpenFailsClosed(t *testing.T) {
	boom := errors.New("boom")
	got, err := KeepOpen(context.Background(), &fakeOpen{err: boom}, found(1))
	if !errors.Is(err, boom) || len(got) != 0 {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestKeepOpenWithNothingFoundSkipsTheLookup(t *testing.T) {
	gate := &fakeOpen{err: errors.New("不該被呼叫")}
	got, err := KeepOpen(context.Background(), gate, nil)
	if err != nil || got != nil {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

// 同一家店重複出現只留第一筆，也只問一次。
func TestKeepOpenDeduplicatesStoresKeepingTheFirst(t *testing.T) {
	gate := &fakeOpen{open: map[int64]struct{}{10: {}, 20: {}}}
	in := found(10, 20, 10)
	in[2].Score = 7
	got, err := KeepOpen(context.Background(), gate, in)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int64{10, 20}; !reflect.DeepEqual(ids(got), want) {
		t.Fatalf("候選 = %v，要 %v", ids(got), want)
	}
	if got[0].Score == 7 {
		t.Fatal("重複時要留第一筆")
	}
	if want := []int64{10, 20}; !reflect.DeepEqual(gate.got, want) {
		t.Fatalf("查了 %v，要 %v", gate.got, want)
	}
}
