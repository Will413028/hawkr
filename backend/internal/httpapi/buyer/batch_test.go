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
		ids[i] = "0190a000-0000-7000-8000-000000000001"
	}
	parsed, err := batchUUIDs(ids, "product_ids")
	if err != nil || len(parsed) != buyer.MaxBatchSize+1 {
		t.Fatalf("parse %d ids: len=%d err=%v", len(ids), len(parsed), err)
	}
	if _, err := buyer.New(buyer.Deps{Now: time.Now}).BatchCheckFavorites(context.Background(), 1, parsed); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("module limit err=%v", err)
	}
	for _, bad := range []string{"0", "1", "00000000000000000000000000000001", "urn:uuid:0190a000-0000-7000-8000-000000000001"} {
		if _, err := batchUUIDs([]string{"0190a000-0000-7000-8000-000000000001", bad}, "product_ids"); err == nil {
			t.Fatalf("%q must fail", bad)
		}
	}
	if storeIDs, err := batchIDs([]string{"9007199254740993"}, "store_ids"); err != nil || len(storeIDs) != 1 || storeIDs[0] != 9007199254740993 {
		t.Fatalf("ids=%v err=%v", storeIDs, err)
	}
}
