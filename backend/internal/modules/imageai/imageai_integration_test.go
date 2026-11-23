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
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/llm"
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

func TestConsumeCountsAgainstANewerExistingPeriodWithoutMovingItBackwards(t *testing.T) {
	t.Parallel()
	d := pg.NewDB(t, testdb.Migrations(migrate.Module{Name: "imageai", Dir: imageai.Migrations()}))
	ctx := context.Background()
	now := wednesday.AddDate(0, 0, 7)
	m := imageai.New(imageai.Deps{DB: d, Now: func() time.Time { return now }})
	setLimit(t, d, 50, 2)
	if _, err := m.ConsumeQuota(ctx, 50); err != nil {
		t.Fatal(err)
	}
	newer := imageaiWeekOf(now)
	now = wednesday
	u, err := m.ConsumeQuota(ctx, 50)
	if err != nil || u.Used != 2 || !u.PeriodStart.Equal(newer) {
		t.Fatalf("時鐘較慢的 pod 要累加到較新的那週：%+v %v", u, err)
	}
	if _, err := m.ConsumeQuota(ctx, 50); !errors.Is(err, imageai.ErrQuotaExhausted) {
		t.Fatalf("較新的那週用完要拒絕：%v", err)
	}
	var start time.Time
	if err := d.Pool().QueryRow(ctx, `SELECT period_start FROM imageai.usage WHERE store_id = 50`).Scan(&start); err != nil || !start.Equal(newer) {
		t.Fatalf("period_start 不得往回移：%v err = %v", start, err)
	}
	if err := m.RefundQuota(ctx, 50, u.PeriodStart); err != nil || used(t, d, 50) != 1 {
		t.Fatalf("用回傳的週退款：%v used = %d", err, used(t, d, 50))
	}
}

type stubGuard struct{ err error }

func (g stubGuard) LockNotRetired(context.Context, int64) error { return g.err }

type stubGen struct {
	res   llm.ImageResult
	err   error
	calls int
}

func (g *stubGen) GenerateImage(context.Context, llm.ImageRequest) (llm.ImageResult, error) {
	g.calls++
	return g.res, g.err
}

func setupTransform(t *testing.T, guardErr error, gen *stubGen) (*db.DB, *imageai.Module) {
	t.Helper()
	t.Parallel()
	d := pg.NewDB(t, testdb.Migrations(migrate.Module{Name: "imageai", Dir: imageai.Migrations()}))
	return d, imageai.New(imageai.Deps{DB: d, Now: func() time.Time { return wednesday }, Seller: stubGuard{err: guardErr}, Images: gen})
}

func pngInput(storeID int64) imageai.TransformInput {
	return imageai.TransformInput{StoreID: storeID, Image: append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 32)...), MimeType: "image/png", Prompt: "白底"}
}

func TestTransformChargesTheQuotaOnSuccessAndRefundsWhenTheModelFails(t *testing.T) {
	gen := &stubGen{res: llm.ImageResult{Image: []byte("out"), MimeType: "image/png"}}
	d, m := setupTransform(t, nil, gen)
	ctx := context.Background()
	out, err := m.Transform(ctx, pngInput(1))
	if err != nil || string(out.Image) != "out" || out.Usage.Used != 1 || out.Usage.Limit != imageai.DefaultWeeklyLimit || used(t, d, 1) != 1 {
		t.Fatalf("out = %+v err = %v used = %d", out, err, used(t, d, 1))
	}
	gen.res, gen.err = llm.ImageResult{}, errors.New("provider down")
	if _, err := m.Transform(ctx, pngInput(1)); err == nil {
		t.Fatal("模型失敗要回錯誤")
	}
	if used(t, d, 1) != 1 {
		t.Fatalf("失敗的那一次要退回，used = %d", used(t, d, 1))
	}
	gen.err, gen.res = nil, llm.ImageResult{FinishReason: "IMAGE_SAFETY"}
	if _, err := m.Transform(ctx, pngInput(1)); err == nil || used(t, d, 1) != 1 {
		t.Fatalf("被擋下也要退回：%v used = %d", err, used(t, d, 1))
	}
}

func TestTransformStopsAtTheQuotaWithoutCallingTheModel(t *testing.T) {
	gen := &stubGen{res: llm.ImageResult{Image: []byte("out"), MimeType: "image/png"}}
	d, m := setupTransform(t, nil, gen)
	setLimit(t, d, 1, 1)
	ctx := context.Background()
	if _, err := m.Transform(ctx, pngInput(1)); err != nil {
		t.Fatal(err)
	}
	_, err := m.Transform(ctx, pngInput(1))
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Reason != apperr.ReasonImageQuotaExceeded || gen.calls != 1 || used(t, d, 1) != 1 {
		t.Fatalf("err = %v calls = %d used = %d", err, gen.calls, used(t, d, 1))
	}
}

func TestTransformForARetiredStoreCreatesNoUsageRow(t *testing.T) {
	gen := &stubGen{res: llm.ImageResult{Image: []byte("out"), MimeType: "image/png"}}
	d, m := setupTransform(t, apperr.NotFound("已退場"), gen)
	_, err := m.Transform(context.Background(), pngInput(1))
	if apperr.KindOf(err) != apperr.KindPermissionDenied || gen.calls != 0 {
		t.Fatalf("err = %v calls = %d", err, gen.calls)
	}
	var n int
	if err := d.Pool().QueryRow(context.Background(), `SELECT count(*) FROM imageai.usage`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("退場的店不能重建用量列：%d %v", n, err)
	}
}

func TestCurrentUsageReadsWithoutConsuming(t *testing.T) {
	d, m := setup(t)
	ctx := context.Background()
	u, err := m.CurrentUsage(ctx, 1)
	if err != nil || u.Used != 0 || u.Limit != imageai.DefaultWeeklyLimit || used(t, d, 1) != 0 {
		t.Fatalf("u = %+v err = %v", u, err)
	}
	if _, err := m.ConsumeQuota(ctx, 1); err != nil {
		t.Fatal(err)
	}
	setLimit(t, d, 1, 3)
	if u, err := m.CurrentUsage(ctx, 1); err != nil || u.Used != 1 || u.Limit != 3 || used(t, d, 1) != 1 {
		t.Fatalf("u = %+v err = %v", u, err)
	}
	d.Pool().Close()
	if _, err := m.CurrentUsage(ctx, 1); err == nil {
		t.Fatal("資料庫錯誤要往上回，不能當成預設值")
	}
}

func TestListStylesReturnsEnabledNonDefaultStylesOfTheCategoryInOrder(t *testing.T) {
	d, m := setup(t)
	ctx := context.Background()
	ins := func(id, name, category string, order int, enabled bool, thumb string) {
		exec(t, d, `INSERT INTO imageai.styles (id, name, label, prompt, category, display_order, enabled, thumbnail_key, created_at, updated_at)
			VALUES ($1, $2, $2, 'p-' || $2, $3, $4, $5, $6, now(), now())`, id, name, category, order, enabled, thumb)
	}
	ins("019e9b00-0000-7000-8000-000000000001", "b", "PRODUCT", 1, true, "styles/b.jpg")
	ins("019e9b00-0000-7000-8000-000000000002", "a", "PRODUCT", 2, true, "")
	ins("019e9b00-0000-7000-8000-000000000003", "off", "PRODUCT", 0, false, "")
	ins("019e9b00-0000-7000-8000-000000000004", "post-only", "POST", 0, true, "")
	list, err := m.ListStyles(ctx, imageai.CategoryProduct)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, s := range list.Styles {
		names = append(names, s.Name)
	}
	if len(names) != 2 || names[0] != "b" || names[1] != "a" || list.Styles[0].ThumbnailKey != "styles/b.jpg" || list.Styles[0].DisplayOrder != 1 {
		t.Fatalf("風格 = %+v", list.Styles)
	}
	var want string
	if err := d.Pool().QueryRow(ctx, `SELECT prompt FROM imageai.styles WHERE category = 'PRODUCT' AND is_default`).Scan(&want); err != nil {
		t.Fatal(err)
	}
	if list.DefaultPrompt != want || want == "" {
		t.Fatalf("DefaultPrompt = %q，預設風格的 prompt = %q", list.DefaultPrompt, want)
	}
	post, err := m.ListStyles(ctx, imageai.CategoryPost)
	if err != nil || len(post.Styles) != 1 || post.Styles[0].Name != "post-only" {
		t.Fatalf("POST = %+v err = %v", post, err)
	}
	if _, err := m.ListStyles(ctx, ""); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("空用途要回 InvalidInput：%v", err)
	}
	d.Pool().Close()
	if _, err := m.ListStyles(ctx, imageai.CategoryProduct); err == nil {
		t.Fatal("資料庫錯誤要往上回")
	}
}
