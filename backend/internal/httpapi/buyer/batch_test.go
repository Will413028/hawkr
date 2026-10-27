package buyer

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// handler 只解析 ID；上限只在模組檢查一次（buyer.MaxBatchSize），超過時的錯誤來自模組。
func TestBatchIDsParsesAndModuleOwnsTheLimit(t *testing.T) {
	ids := make([]string, buyer.MaxBatchSize+1)
	for i := range ids {
		ids[i] = "1"
	}
	parsed, err := batchIDs(ids, "product_ids")
	if err != nil || len(parsed) != buyer.MaxBatchSize+1 {
		t.Fatalf("parse %d ids: len=%d err=%v", len(ids), len(parsed), err)
	}
	if _, err := buyer.New(buyer.Deps{Now: time.Now}).BatchCheckFavorites(context.Background(), 1, parsed); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("module limit err=%v", err)
	}
	if _, err := batchIDs([]string{"1", "0"}, "product_ids"); err == nil {
		t.Fatal("nonpositive ID must fail")
	}
	parsed, err = batchIDs([]string{"9007199254740993"}, "product_ids")
	if err != nil || len(parsed) != 1 || parsed[0] != 9007199254740993 {
		t.Fatalf("ids=%v err=%v", parsed, err)
	}
}
