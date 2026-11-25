package stock

import (
	"context"
	"errors"
	"strings"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog/internal/model"
)

func TestAdjustRejectsInvalidInputBeforeTouchingTheDatabase(t *testing.T) {
	t.Parallel()
	s := New(Deps{})
	sku := uuid.NewV7()
	cases := map[string]struct {
		store int64
		key   string
		sku   uuid.UUID
		delta int32
	}{
		"no store":       {0, "k", sku, 1},
		"zero delta":     {501, "k", sku, 0},
		"nil sku":        {501, "k", uuid.Nil(), 1},
		"empty key":      {501, "", sku, 1},
		"padded key":     {501, " k", sku, 1},
		"key too long":   {501, strings.Repeat("k", maxKeyRunes+1), sku, 1},
		"negative store": {-1, "k", sku, 1},
	}
	for name, c := range cases {
		if err := s.Adjust(context.Background(), c.store, c.key, c.sku, c.delta); !errors.Is(err, model.ErrInvalidStockInput) {
			t.Errorf("%s: err = %v, want ErrInvalidStockInput", name, err)
		}
	}
}
