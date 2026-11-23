//go:build integration

package imageai_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/backend/internal/modules/imageai"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

var wednesday = time.Date(2026, 11, 25, 10, 0, 0, 0, twtime.Taipei)

func setup(t *testing.T) (*db.DB, *imageai.Module) {
	t.Helper()
	t.Parallel()
	d := pg.NewDB(t, testdb.Migrations(migrate.Module{Name: "imageai", Dir: imageai.Migrations()}))
	return d, imageai.New(imageai.Deps{DB: d, Now: func() time.Time { return wednesday }})
}

func exec(t *testing.T, d *db.DB, sql string, args ...any) {
	t.Helper()
	if _, err := d.Pool().Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func used(t *testing.T, d *db.DB, storeID int64) int {
	t.Helper()
	var n int
	err := d.Pool().QueryRow(context.Background(), `SELECT COALESCE(SUM(used), 0) FROM imageai.usage WHERE store_id = $1`, storeID).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func setLimit(t *testing.T, d *db.DB, storeID int64, limit int) {
	t.Helper()
	exec(t, d, `INSERT INTO imageai.quotas (store_id, weekly_limit, created_at, updated_at) VALUES ($1, $2, now(), now())`, storeID, limit)
}

func TestConsumeUsesTheDefaultLimitAndThePerStoreLimitOverridesIt(t *testing.T) {
	d, m := setup(t)
	ctx := context.Background()
	u, err := m.ConsumeQuota(ctx, 1)
	if err != nil || u.Used != 1 || u.Limit != imageai.DefaultWeeklyLimit {
		t.Fatalf("u = %+v err = %v", u, err)
	}
	if want := time.Date(2026, 11, 23, 0, 0, 0, 0, twtime.Taipei); !u.PeriodStart.Equal(want) || !u.ResetTime.Equal(want.AddDate(0, 0, 7)) {
		t.Fatalf("週界線 = %v ~ %v", u.PeriodStart, u.ResetTime)
	}
	setLimit(t, d, 2, 1)
	if _, err := m.ConsumeQuota(ctx, 2); err != nil {
		t.Fatal(err)
	}
	if u, err := m.ConsumeQuota(ctx, 2); !errors.Is(err, imageai.ErrQuotaExhausted) || u.Used != 1 || u.Limit != 1 {
		t.Fatalf("個別上限 1 的第二次：%+v %v", u, err)
	}
	setLimit(t, d, 3, 0)
	if _, err := m.ConsumeQuota(ctx, 3); !errors.Is(err, imageai.ErrQuotaExhausted) {
		t.Fatalf("上限 0 要直接拒絕：%v", err)
	}
	if used(t, d, 3) != 0 {
		t.Fatal("上限 0 不該建用量列")
	}
}

func TestConcurrentConsumesOnTheLastUnitSucceedExactlyOnce(t *testing.T) {
	d, m := setup(t)
	ctx := context.Background()
	setLimit(t, d, 7, 3)
	for range 2 {
		if _, err := m.ConsumeQuota(ctx, 7); err != nil {
			t.Fatal(err)
		}
	}
	const workers = 16
	var (
		wg       sync.WaitGroup
		start    = make(chan struct{})
		mu       sync.Mutex
		ok, deny int
		other    []error
	)
	for range workers {
		wg.Go(func() {
			<-start
			_, err := m.ConsumeQuota(ctx, 7)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				ok++
			case errors.Is(err, imageai.ErrQuotaExhausted):
				deny++
			default:
				other = append(other, err)
			}
		})
	}
	close(start)
	wg.Wait()
	if len(other) > 0 || ok != 1 || deny != workers-1 {
		t.Fatalf("成功 %d、拒絕 %d、其他錯誤 %v；預期 1 與 %d", ok, deny, other, workers-1)
	}
	if got := used(t, d, 7); got != 3 {
		t.Fatalf("used = %d，預期剛好等於上限 3", got)
	}
}

func TestRefundGivesBackOneUnitAndNeverGoesBelowZero(t *testing.T) {
	d, m := setup(t)
	ctx := context.Background()
	u, err := m.ConsumeQuota(ctx, 9)
	if err != nil || used(t, d, 9) != 1 {
		t.Fatalf("扣：%+v %v", u, err)
	}
	if err := m.RefundQuota(ctx, 9, u.PeriodStart); err != nil {
		t.Fatal(err)
	}
	if got := used(t, d, 9); got != 0 {
		t.Fatalf("退回後 used = %d，預期 0", got)
	}
	if err := m.RefundQuota(ctx, 9, u.PeriodStart); !errors.Is(err, imageai.ErrNothingToRefund) || used(t, d, 9) != 0 {
		t.Fatalf("多退一次要回 ErrNothingToRefund 且不得變負：%v used=%d", err, used(t, d, 9))
	}
	if err := m.RefundQuota(ctx, 404, u.PeriodStart); !errors.Is(err, imageai.ErrNothingToRefund) {
		t.Fatalf("沒有用量列要回 ErrNothingToRefund：%v", err)
	}
	setLimit(t, d, 10, 1)
	u, _ = m.ConsumeQuota(ctx, 10)
	if _, err := m.ConsumeQuota(ctx, 10); !errors.Is(err, imageai.ErrQuotaExhausted) {
		t.Fatalf("用完要拒絕：%v", err)
	}
	if err := m.RefundQuota(ctx, 10, u.PeriodStart); err != nil {
		t.Fatal(err)
	}
	if _, err := m.ConsumeQuota(ctx, 10); err != nil {
		t.Fatalf("退回後要能再扣：%v", err)
	}
}

func TestStoreRetiredDeletesThatStoresQuotaAndUsageOnly(t *testing.T) {
	d, m := setup(t)
	ctx := context.Background()
	bus := events.NewBus()
	events.Subscribe(bus, m.OnStoreRetired)
	for _, id := range []int64{20, 21} {
		setLimit(t, d, id, 5)
		if _, err := m.ConsumeQuota(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	retire := func() error {
		return d.InTx(ctx, func(ctx context.Context) error { return events.Publish(ctx, bus, imageai.StoreRetired{StoreID: 20}) })
	}
	for range 2 {
		if err := retire(); err != nil {
			t.Fatal(err)
		}
	}
	count := func(table string, id int64) (n int) {
		if err := d.Pool().QueryRow(ctx, `SELECT count(*) FROM imageai.`+table+` WHERE store_id = $1`, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if count("quotas", 20) != 0 || count("usage", 20) != 0 {
		t.Fatal("退場的店家還有額度或用量")
	}
	if count("quotas", 21) != 1 || count("usage", 21) != 1 {
		t.Fatal("別家店的額度或用量被刪了")
	}
}

func TestDefaultStyleCannotBeDisabled(t *testing.T) {
	d, _ := setup(t)
	_, err := d.Pool().Exec(context.Background(), `UPDATE imageai.styles SET enabled = FALSE WHERE is_default`)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("停用預設風格要被 CHECK 擋下：%v", err)
	}
}

func TestEveryCategorySeedsExactlyOneDefaultStyleAndAnotherDefaultIsRejected(t *testing.T) {
	d, _ := setup(t)
	ctx := context.Background()
	rows, err := d.Pool().Query(ctx, `SELECT category, count(*) FROM imageai.styles WHERE is_default AND enabled GROUP BY category`)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for rows.Next() {
		var c string
		var n int
		if err := rows.Scan(&c, &n); err != nil {
			t.Fatal(err)
		}
		got[c] = n
	}
	rows.Close()
	if got["PRODUCT"] != 1 || got["POST"] != 1 || len(got) != 2 {
		t.Fatalf("預設風格 = %v", got)
	}
	_, err = d.Pool().Exec(ctx, `INSERT INTO imageai.styles (id, name, label, prompt, category, is_default, created_at, updated_at)
		VALUES ('019e9a00-0000-7000-8000-0000000000ff', 'other', 'x', 'p', 'PRODUCT', TRUE, now(), now())`)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("同用途第二個預設要被唯一索引擋下：%v", err)
	}
}

func TestRefundIgnoresTheCallersTransaction(t *testing.T) {
	d, m := setup(t)
	ctx := context.Background()
	u, err := m.ConsumeQuota(ctx, 30)
	if err != nil {
		t.Fatal(err)
	}
	rollback := errors.New("rollback")
	err = d.InTx(ctx, func(ctx context.Context) error {
		if err := m.RefundQuota(ctx, 30, u.PeriodStart); err != nil {
			return err
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("err = %v", err)
	}
	if got := used(t, d, 30); got != 0 {
		t.Fatalf("呼叫端 transaction 回滾後退款仍要生效：used = %d", got)
	}
}

func TestUsageKeepsOneRowPerStoreAndResetsInANewWeek(t *testing.T) {
	t.Parallel()
	d := pg.NewDB(t, testdb.Migrations(migrate.Module{Name: "imageai", Dir: imageai.Migrations()}))
	ctx := context.Background()
	now := wednesday
	m := imageai.New(imageai.Deps{DB: d, Now: func() time.Time { return now }})
	setLimit(t, d, 40, 2)
	for range 2 {
		if _, err := m.ConsumeQuota(ctx, 40); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := m.ConsumeQuota(ctx, 40); !errors.Is(err, imageai.ErrQuotaExhausted) {
		t.Fatalf("用完要拒絕：%v", err)
	}
	first := wednesday
	now = wednesday.AddDate(0, 0, 7)
	u, err := m.ConsumeQuota(ctx, 40)
	if err != nil || u.Used != 1 {
		t.Fatalf("新的一週要重設為 1：%+v %v", u, err)
	}
	var rows int
	if err := d.Pool().QueryRow(ctx, `SELECT count(*) FROM imageai.usage WHERE store_id = 40`).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("每店一列：rows = %d err = %v", rows, err)
	}
	if err := m.RefundQuota(ctx, 40, imageaiWeekOf(first)); !errors.Is(err, imageai.ErrNothingToRefund) || used(t, d, 40) != 1 {
		t.Fatalf("退上一週的額度不得動到本週：%v used = %d", err, used(t, d, 40))
	}
}

func imageaiWeekOf(t time.Time) time.Time {
	d := twtime.Midnight(t)
	return time.Date(d.Year(), d.Month(), d.Day()-(int(d.Weekday())+6)%7, 0, 0, 0, 0, twtime.Taipei)
}
