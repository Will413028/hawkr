package retention

import (
	"context"
	"testing"
	"uuid"
)

type markRecorder struct{ calls [][]uuid.UUID }

func (m *markRecorder) MarkDeleted(_ context.Context, ids []uuid.UUID) error {
	m.calls = append(m.calls, ids)
	return nil
}

func TestRetireAssetsMarksEveryOwnedCopyOnceAndSkipsMediaWhenEmpty(t *testing.T) {
	a, b := uuid.NewV7(), uuid.NewV7()
	m := &markRecorder{}
	s := New(Deps{Media: m})
	if err := s.retireAssets(t.Context(), [][]uuid.UUID{{a, b}, {}, {a}}); err != nil {
		t.Fatal(err)
	}
	if len(m.calls) != 1 || len(m.calls[0]) != 2 {
		t.Fatalf("calls = %v，預期一次呼叫、兩個去重的資產", m.calls)
	}
	m.calls = nil
	if err := s.retireAssets(t.Context(), [][]uuid.UUID{{}, nil}); err != nil || len(m.calls) != 0 {
		t.Fatalf("沒有資產時不呼叫 media：%v %v", err, m.calls)
	}
}
