package buyer

import (
	"context"
	"math"
	"testing"

	"connectrpc.com/connect"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/buyer/v1alpha1"
	model "github.com/Will413028/hawkr/backend/internal/modules/buyer"
)

type fakeAdmin struct{ stats model.AdminStats }

func (f *fakeAdmin) StatsForAdmin(context.Context) (model.AdminStats, error) { return f.stats, nil }

func TestAdminStatsOverflow(t *testing.T) {
	h := NewAdmin(&fakeAdmin{stats: model.AdminStats{Total: math.MaxInt32 + 1}})
	if _, err := h.GetBuyerStats(context.Background(), connect.NewRequest(&adminpb.GetBuyerStatsRequest{})); err == nil {
		t.Fatal("int32 overflow accepted")
	}
}
