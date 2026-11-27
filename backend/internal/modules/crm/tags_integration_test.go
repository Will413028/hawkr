//go:build integration

package crm_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/backend/internal/modules/crm"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func (f *fixture) mustAddTags(storeID, buyerID int64, names ...string) {
	f.t.Helper()
	if _, err := f.m.AddCustomerTags(context.Background(), storeID, buyerID, names); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) tagNames(storeID int64) []string {
	f.t.Helper()
	tags, err := f.m.ListTags(context.Background(), storeID)
	if err != nil {
		f.t.Fatal(err)
	}
	out := make([]string, len(tags))
	for i, tg := range tags {
		out[i] = tg.Name
	}
	return out
}

func wantKind(t *testing.T, err error, kind apperr.Kind, reason crm.Reason) {
	t.Helper()
	if apperr.KindOf(err) != kind || crm.ReasonOf(err) != reason {
		t.Fatalf("err=%v kind=%v reason=%q，預期 %v、%q", err, apperr.KindOf(err), crm.ReasonOf(err), kind, reason)
	}
}

// CD3：並行以同名貼標，只會有一個標籤；每位客戶都貼上了它。
func TestAddCustomerTagsConcurrentSameNameCreatesOneTag(t *testing.T) {
	t.Parallel()
	f := setup(t)
	const buyers = 16
	for i := 0; i < buyers; i++ {
		f.mustOrder(fmt.Sprintf("evt-%d", i), fmt.Sprintf("O%d", i), storeA, buyer1+int64(i), 100)
	}
	f.exec(`CREATE FUNCTION crm.slow_tag_insert() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_sleep(0.2); RETURN NEW; END $$`)
	f.exec(`CREATE TRIGGER slow_tag_insert BEFORE INSERT ON crm.tags FOR EACH ROW EXECUTE FUNCTION crm.slow_tag_insert()`)
	errs := parallel(buyers, func(i int) error {
		return errOnly(f.m.AddCustomerTags(context.Background(), storeA, buyer1+int64(i), []string{"常客"}))
	})
	noErrors(t, errs)
	if got := f.tagNames(storeA); len(got) != 1 || got[0] != "常客" {
		t.Fatalf("標籤 %v，預期只有一個「常客」", got)
	}
	if n := f.count("crm.customer_tags"); n != buyers {
		t.Fatalf("貼標 %d 筆，預期 %d", n, buyers)
	}
}

// CD5：AddCustomerTags 是原子的。這裡在第二個標籤的 INSERT 注入失敗（觸發器），斷言第一個標籤也沒有留下
// （「N 件事都成立」的斷言鑑別力為零：要注入其中一件的失敗，再斷言其餘的也沒發生）。
func TestAddCustomerTagsIsAtomicWhenALaterStepFails(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.mustOrder("evt-1", "O1", storeA, buyer1, 100)
	f.exec(`CREATE FUNCTION crm.fail_on_boom() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN IF NEW.name = 'boom' THEN RAISE EXCEPTION 'injected failure'; END IF; RETURN NEW; END $$`)
	f.exec(`CREATE TRIGGER fail_on_boom BEFORE INSERT ON crm.tags FOR EACH ROW EXECUTE FUNCTION crm.fail_on_boom()`)
	_, err := f.m.AddCustomerTags(context.Background(), storeA, buyer1, []string{"alpha", "boom", "gamma"})
	if err == nil || !strings.Contains(err.Error(), "injected failure") {
		t.Fatalf("err=%v，預期注入的失敗", err)
	}
	if n := f.count("crm.tags"); n != 0 {
		t.Fatalf("失敗後留下 %d 個標籤（alpha 在失敗之前已寫入，必須一起回滾）", n)
	}
	if n := f.count("crm.customer_tags"); n != 0 {
		t.Fatalf("失敗後留下 %d 筆貼標", n)
	}
}

// 含一個超長名稱：整批無寫入，回 InvalidInput。
func TestAddCustomerTagsWithOneTooLongNameWritesNothing(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.mustOrder("evt-1", "O1", storeA, buyer1, 100)
	_, err := f.m.AddCustomerTags(context.Background(), storeA, buyer1, []string{"ok", strings.Repeat("長", crm.MaxTagNameRunes+1)})
	wantKind(t, err, apperr.KindInvalidInput, "")
	if f.count("crm.tags") != 0 || f.count("crm.customer_tags") != 0 {
		t.Fatal("超長名稱使整批的其他標籤被寫入")
	}
}

func TestAddCustomerTagsInputValidation(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.mustOrder("evt-1", "O1", storeA, buyer1, 100)
	ctx := context.Background()
	var many []string
	for i := 0; i <= crm.MaxTagsPerCall; i++ {
		many = append(many, fmt.Sprintf("t%d", i))
	}
	for name, names := range map[string][]string{"空清單": nil, "超過單次上限": many, "空白名稱": {"  "}} {
		if _, err := f.m.AddCustomerTags(ctx, storeA, buyer1, names); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：err=%v，預期 InvalidInput", name, err)
		}
	}
	f.mustAddTags(storeA, buyer1, " 常客", "常客 ", "常客")
	if got := f.tagNames(storeA); len(got) != 1 {
		t.Fatalf("標籤 %v，預期去重成一個", got)
	}
	f.mustAddTags(storeA, buyer1, "常客")
	if n := f.count("crm.customer_tags"); n != 1 {
		t.Fatalf("貼標 %d 筆，預期 1", n)
	}
}

// 只有客戶才能貼標：沒有客戶列回 NotFound，而且連標籤都沒有建立。
func TestAddCustomerTagsToNonCustomerIsNotFoundAndCreatesNothing(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, err := f.m.AddCustomerTags(context.Background(), storeA, buyer1, []string{"常客"})
	wantKind(t, err, apperr.KindNotFound, crm.ReasonCustomerNotFound)
	if f.count("crm.tags") != 0 {
		t.Fatal("非客戶的貼標建立了標籤")
	}
}

// CD4：customer_tags 以他店的 tag 插入 → FK 失敗（標籤與客戶同店由 DB 保證）。
func TestCustomerTagsRejectsAnotherStoresTag(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.mustOrder("evt-1", "O1", storeA, buyer1, 100)
	tagB, err := f.m.CreateTag(context.Background(), storeB, "他店的標籤")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.db.Pool().Exec(context.Background(),
		"INSERT INTO crm.customer_tags (store_id, buyer_id, tag_id, created_at) VALUES ($1, $2, $3, now())", storeA, buyer1, tagB.ID)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		t.Fatalf("err=%v，預期外鍵違反 23503", err)
	}
	tagA, err := f.m.CreateTag(context.Background(), storeA, "本店")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.db.Pool().Exec(context.Background(),
		"INSERT INTO crm.customer_tags (store_id, buyer_id, tag_id, created_at) VALUES ($1, $2, $3, now())", storeA, buyer2, tagA.ID)
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		t.Fatalf("非客戶貼標 err=%v，預期外鍵違反 23503", err)
	}
}

func TestCreateRenameDeleteTag(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.mustOrder("evt-1", "O1", storeA, buyer1, 100)
	a, err := f.m.CreateTag(ctx, storeA, "  甲  ")
	if err != nil || a.Name != "甲" {
		t.Fatalf("tag=%+v err=%v（名稱要 trim）", a, err)
	}
	b, err := f.m.CreateTag(ctx, storeA, "乙")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.m.CreateTag(ctx, storeA, "甲")
	wantKind(t, err, apperr.KindAlreadyExists, crm.ReasonTagNameTaken)
	wantKind(t, errOnly(f.m.RenameTag(ctx, storeA, b.ID, "甲")), apperr.KindAlreadyExists, crm.ReasonTagNameTaken)
	if _, err := f.m.CreateTag(ctx, storeB, "甲"); err != nil {
		t.Fatalf("別家店同名被擋：%v", err)
	}
	if _, err := f.m.RenameTag(ctx, storeA, b.ID, "乙"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.RenameTag(ctx, storeA, b.ID, "丙"); err != nil {
		t.Fatal(err)
	}
	wantKind(t, errOnly(f.m.RenameTag(ctx, storeB, b.ID, "丁")), apperr.KindNotFound, crm.ReasonTagNotFound)
	wantKind(t, errOnly(f.m.RenameTag(ctx, storeA, uuid.UUID{}, "丁")), apperr.KindNotFound, crm.ReasonTagNotFound)
	wantKind(t, f.m.DeleteTag(ctx, storeB, b.ID), apperr.KindNotFound, crm.ReasonTagNotFound)
	f.mustAddTags(storeA, buyer1, "甲", "丙")
	if err := f.m.DeleteTag(ctx, storeA, b.ID); err != nil {
		t.Fatal(err)
	}
	c, err := f.m.GetCustomer(ctx, storeA, buyer1)
	if err != nil || len(c.Tags) != 1 || c.Tags[0].Name != "甲" {
		t.Fatalf("customer=%+v err=%v，預期只剩「甲」", c, err)
	}
	wantKind(t, f.m.DeleteTag(ctx, storeA, b.ID), apperr.KindNotFound, crm.ReasonTagNotFound)
}

func TestRemoveCustomerTagIsIdempotent(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.mustOrder("evt-1", "O1", storeA, buyer1, 100)
	tg, err := f.m.CreateTag(ctx, storeA, "甲")
	if err != nil {
		t.Fatal(err)
	}
	f.mustAddTags(storeA, buyer1, "甲")
	for i := 0; i < 2; i++ {
		if _, err := f.m.RemoveCustomerTag(ctx, storeA, buyer1, tg.ID); err != nil {
			t.Fatalf("第 %d 次移除：%v", i, err)
		}
	}
	if n := f.count("crm.customer_tags"); n != 0 {
		t.Fatalf("貼標 %d 筆", n)
	}
	if got := f.tagNames(storeA); len(got) != 1 {
		t.Fatalf("移除關聯不該刪標籤：%v", got)
	}
}

// W4：每店 200 個標籤、每位客戶 20 個；超過回 FailedPrecondition（CRM_TAG_LIMIT），並且整批沒有寫入。
func TestTagLimits(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.mustOrder("evt-1", "O1", storeA, buyer1, 100)
	f.exec(`INSERT INTO crm.tags (id, store_id, name, created_at, updated_at)
		SELECT gen_random_uuid(), $1, 'fill' || g, now(), now() FROM generate_series(1, $2) g`, storeA, crm.MaxTagsPerStore-1)
	if _, err := f.m.CreateTag(ctx, storeA, "最後一個"); err != nil {
		t.Fatalf("第 %d 個標籤被擋：%v", crm.MaxTagsPerStore, err)
	}
	_, err := f.m.CreateTag(ctx, storeA, "多一個")
	wantKind(t, err, apperr.KindFailedPrecondition, crm.ReasonTagLimit)
	_, err = f.m.AddCustomerTags(ctx, storeA, buyer1, []string{"最後一個", "又一個新的"})
	wantKind(t, err, apperr.KindFailedPrecondition, crm.ReasonTagLimit)
	if n := f.count("crm.customer_tags"); n != 0 {
		t.Fatalf("超過每店上限的整批仍貼了 %d 筆", n)
	}
	f.mustOrder("evt-2", "O2", storeB, buyer1, 100)
	var names []string
	for i := 0; i < crm.MaxTagsPerCustomer; i++ {
		names = append(names, fmt.Sprintf("c%d", i))
	}
	f.mustAddTags(storeB, buyer1, names...)
	_, err = f.m.AddCustomerTags(ctx, storeB, buyer1, []string{"c0", "超過"})
	wantKind(t, err, apperr.KindFailedPrecondition, crm.ReasonTagLimit)
	if got := f.tagNames(storeB); len(got) != crm.MaxTagsPerCustomer {
		t.Fatalf("店 B 有 %d 個標籤，預期 %d（超過上限的整批要連新標籤一起回滾）", len(got), crm.MaxTagsPerCustomer)
	}
	f.mustAddTags(storeB, buyer1, names...)
}

func (f *fixture) slowInsert(table string) {
	f.t.Helper()
	f.exec(fmt.Sprintf(`CREATE FUNCTION crm.slow_%[1]s_insert() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_sleep(0.3); RETURN NEW; END $$`, table))
	f.exec(fmt.Sprintf(`CREATE TRIGGER slow_insert BEFORE INSERT ON crm.%[1]s FOR EACH ROW EXECUTE FUNCTION crm.slow_%[1]s_insert()`, table))
}

// CR1：AddCustomerTags 以名稱查到標籤 ID 之後、貼標之前，DeleteTag 不能插進來（否則 FK 23503 變成 internal）。
// 貼標的 INSERT 慢 300ms 只拉寬競爭窗口；DeleteTag 在窗口內才開始。兩種順序都合法：貼標先（刪除隨後 cascade）或刪除先（貼標重建同名標籤）。
func TestAddCustomerTagsConcurrentWithDeleteTag(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.mustOrder("evt-1", "O1", storeA, buyer1, 100)
	tag, err := f.m.CreateTag(ctx, storeA, "常客")
	if err != nil {
		t.Fatal(err)
	}
	f.slowInsert("customer_tags")
	errs := parallel(2, func(i int) error {
		if i == 0 {
			return errOnly(f.m.AddCustomerTags(ctx, storeA, buyer1, []string{"常客"}))
		}
		time.Sleep(100 * time.Millisecond)
		return f.m.DeleteTag(ctx, storeA, tag.ID)
	})
	noErrors(t, errs)
	if n := f.scalar("SELECT count(*) FROM crm.customer_tags ct LEFT JOIN crm.tags t ON t.store_id = ct.store_id AND t.id = ct.tag_id WHERE t.id IS NULL"); n != 0 {
		t.Fatalf("%d 筆貼標指向不存在的標籤", n)
	}
}

// CR1：AddCustomerTags 判定「名稱不存在」之後要建立它，RenameTag 不能在這之間把別的標籤改成這個名稱（否則 INSERT 撞名被當成內部錯誤）。
func TestAddCustomerTagsConcurrentWithRenameTag(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.mustOrder("evt-1", "O1", storeA, buyer1, 100)
	old, err := f.m.CreateTag(ctx, storeA, "舊名")
	if err != nil {
		t.Fatal(err)
	}
	f.slowInsert("tags")
	errs := parallel(2, func(i int) error {
		if i == 0 {
			return errOnly(f.m.AddCustomerTags(ctx, storeA, buyer1, []string{"新名"}))
		}
		time.Sleep(100 * time.Millisecond)
		_, err := f.m.RenameTag(ctx, storeA, old.ID, "新名")
		if crm.ReasonOf(err) == crm.ReasonTagNameTaken {
			return nil
		}
		return err
	})
	noErrors(t, errs)
	if got := f.tagNames(storeA); len(got) == 0 {
		t.Fatal("標籤全不見了")
	}
}
