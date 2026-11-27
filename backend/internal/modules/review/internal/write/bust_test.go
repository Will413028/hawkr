package write

import (
	"context"
	"slices"
	"testing"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/revalidate"
)

type tagJobs struct{ args []river.JobArgs }

func (q *tagJobs) Enqueue(_ context.Context, a river.JobArgs, _ *river.InsertOpts) error {
	q.args = append(q.args, a)
	return nil
}

type stores struct {
	st  seller.Store
	err error
}

func (s stores) Store(context.Context, int64) (seller.Store, error) { return s.st, s.err }

var product700 = uuid.UUID{15: 0xbc, 14: 2}

func bustTags(t *testing.T, s stores) []string {
	t.Helper()
	q := &tagJobs{}
	svc := New(Deps{Jobs: q, Stores: s})
	if err := svc.bust(context.Background(), 91, product700); err != nil {
		t.Fatal(err)
	}
	if len(q.args) != 1 {
		t.Fatalf("jobs=%d", len(q.args))
	}
	return q.args[0].(revalidate.Args).Tags
}

// 店家查不到時商品的 tag 仍要排入；查得到時商品與店家的 tag 都在。
func TestBustAlwaysKeepsTheProductTag(t *testing.T) {
	t.Parallel()
	got := bustTags(t, stores{err: apperr.NotFound("no store")})
	if !slices.Equal(got, []string{revalidate.ProductTag(product700)}) {
		t.Fatalf("店家 NotFound 時只剩商品 tag：%v", got)
	}
	got = bustTags(t, stores{st: seller.Store{StoreID: 91, StoreCode: 5}})
	if !slices.Contains(got, revalidate.ProductTag(product700)) || len(got) != 1+len(revalidate.StoreTags(91, 5)) {
		t.Fatalf("商品與店家 tag 都在：%v", got)
	}
}
