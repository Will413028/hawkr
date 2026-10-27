package usecase

import (
	"context"
	"errors"
	"testing"
)

type fakeGcis struct {
	br    *BusinessRegistration
	err   error
	calls int
}

func (f *fakeGcis) Lookup(_ context.Context, _ string) (*BusinessRegistration, error) {
	f.calls++
	return f.br, f.err
}

type fakeCache struct {
	store map[string]*BusinessRegistration
	sets  int
}

func newFakeCache() *fakeCache { return &fakeCache{store: map[string]*BusinessRegistration{}} }

func (c *fakeCache) Get(_ context.Context, ban string) (*BusinessRegistration, bool) {
	br, ok := c.store[ban]
	return br, ok
}

func (c *fakeCache) Set(_ context.Context, ban string, br *BusinessRegistration) {
	c.sets++
	c.store[ban] = br
}

func TestBusinessRegLookup_BadFormat(t *testing.T) {
	uc := NewBusinessRegistrationUseCase(&fakeGcis{}, newFakeCache())
	for _, ban := range []string{"1234567", "123456789", "abcd1234", ""} {
		_, err := uc.Lookup(context.Background(), ban)
		var verr *ValidationError
		if !errors.As(err, &verr) {
			t.Errorf("Lookup(%q): expected *ValidationError, got %v", ban, err)
		}
	}
}

func TestBusinessRegLookup_Found_CachesResult(t *testing.T) {
	gcis := &fakeGcis{br: &BusinessRegistration{Ban: "33330333", BusinessNm: "台積電"}}
	cache := newFakeCache()
	uc := NewBusinessRegistrationUseCase(gcis, cache)

	br, err := uc.Lookup(context.Background(), "33330333")
	if err != nil || br == nil || br.BusinessNm != "台積電" {
		t.Fatalf("got br=%+v err=%v", br, err)
	}
	if cache.sets != 1 {
		t.Errorf("expected found result cached, sets=%d", cache.sets)
	}

	// Second call hits cache, does not call GCIS again.
	if _, err := uc.Lookup(context.Background(), "33330333"); err != nil {
		t.Fatal(err)
	}
	if gcis.calls != 1 {
		t.Errorf("expected cache hit on 2nd call, gcis.calls=%d", gcis.calls)
	}
}

func TestBusinessRegLookup_NotFound_NotCached(t *testing.T) {
	gcis := &fakeGcis{br: nil} // reachable, no such company
	cache := newFakeCache()
	uc := NewBusinessRegistrationUseCase(gcis, cache)

	br, err := uc.Lookup(context.Background(), "00000000")
	if err != nil || br != nil {
		t.Fatalf("expected (nil,nil) for definitive not-found, got br=%+v err=%v", br, err)
	}
	if cache.sets != 0 {
		t.Errorf("not-found must not be cached, sets=%d", cache.sets)
	}
}

func TestBusinessRegLookup_Transient_Propagates(t *testing.T) {
	sentinel := errors.New("gcis timeout")
	gcis := &fakeGcis{err: sentinel}
	cache := newFakeCache()
	uc := NewBusinessRegistrationUseCase(gcis, cache)

	_, err := uc.Lookup(context.Background(), "33330333")
	if !errors.Is(err, sentinel) {
		t.Errorf("expected transient error to propagate, got %v", err)
	}
	if cache.sets != 0 {
		t.Errorf("transient error must not cache, sets=%d", cache.sets)
	}
}
