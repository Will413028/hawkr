//go:build integration

package crm_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/backend/internal/modules/crm"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func (f *fixture) mustAddTags(storeID, buyerID int64, names ...string) {
	f.t.Helper()
	if err := f.m.AddCustomerTags(context.Background(), storeID, buyerID, names); err != nil {
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
	// 建立標籤的 INSERT 慢 200ms（只拉寬「先查再建」的競爭窗口，不是時間斷言）：沒有序列化時，並行的呼叫都會在第一個 INSERT 完成前查到「不存在」。
	f.exec(`CREATE FUNCTION crm.slow_tag_insert() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_sleep(0.2); RETURN NEW; END $$`)
	f.exec(`CREATE TRIGGER slow_tag_insert BEFORE INSERT ON crm.tags FOR EACH ROW EXECUTE FUNCTION crm.slow_tag_insert()`)
	errs := parallel(buyers, func(i int) error {
		return f.m.AddCustomerTags(context.Background(), storeA, buyer1+int64(i), []string{"常客"})
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
	err := f.m.AddCustomerTags(context.Background(), storeA, buyer1, []string{"alpha", "boom", "gamma"})
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
	err := f.m.AddCustomerTags(context.Background(), storeA, buyer1, []string{"ok", strings.Repeat("長", crm.MaxTagNameRunes+1)})
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
		if err := f.m.AddCustomerTags(ctx, storeA, buyer1, names); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：err=%v，預期 InvalidInput", name, err)
		}
	}
	// 去重與 trim：同一個名稱的不同寫法只貼一次。
	f.mustAddTags(storeA, buyer1, " 常客", "常客 ", "常客")
	if got := f.tagNames(storeA); len(got) != 1 {
		t.Fatalf("標籤 %v，預期去重成一個", got)
	}
	// 冪等：再貼一次不報錯、不重複。
	f.mustAddTags(storeA, buyer1, "常客")
	if n := f.count("crm.customer_tags"); n != 1 {
		t.Fatalf("貼標 %d 筆，預期 1", n)
	}
}

// 只有客戶才能貼標：沒有客戶列回 NotFound，而且連標籤都沒有建立。
func TestAddCustomerTagsToNonCustomerIsNotFoundAndCreatesNothing(t *testing.T) {
	t.Parallel()
	f := setup(t)
	err := f.m.AddCustomerTags(context.Background(), storeA, buyer1, []string{"常客"})
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
	// 只有客戶才有貼標：沒有客戶列的 (store, buyer) 也被 FK 擋下。
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
	// 撞名：建立與改名都回 AlreadyExists。
	_, err = f.m.CreateTag(ctx, storeA, "甲")
	wantKind(t, err, apperr.KindAlreadyExists, crm.ReasonTagNameTaken)
	wantKind(t, f.m.RenameTag(ctx, storeA, b.ID, "甲"), apperr.KindAlreadyExists, crm.ReasonTagNameTaken)
	// 不同店可以同名。
	if _, err := f.m.CreateTag(ctx, storeB, "甲"); err != nil {
		t.Fatalf("別家店同名被擋：%v", err)
	}
	// 改成自己現在的名稱不是撞名。
	if err := f.m.RenameTag(ctx, storeA, b.ID, "乙"); err != nil {
		t.Fatal(err)
	}
	if err := f.m.RenameTag(ctx, storeA, b.ID, "丙"); err != nil {
		t.Fatal(err)
	}
	// 他店的標籤與不存在的標籤都是 NotFound。
	wantKind(t, f.m.RenameTag(ctx, storeB, b.ID, "丁"), apperr.KindNotFound, crm.ReasonTagNotFound)
	wantKind(t, f.m.RenameTag(ctx, storeA, uuid.UUID{}, "丁"), apperr.KindNotFound, crm.ReasonTagNotFound)
	wantKind(t, f.m.DeleteTag(ctx, storeB, b.ID), apperr.KindNotFound, crm.ReasonTagNotFound)
	// 刪除：關聯一起消失。
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
		if err := f.m.RemoveCustomerTag(ctx, storeA, buyer1, tg.ID); err != nil {
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
	// AddCustomerTags 要建立的新標籤會超過每店上限：整批無寫入（已存在的標籤也不貼）。
	err = f.m.AddCustomerTags(ctx, storeA, buyer1, []string{"最後一個", "又一個新的"})
	wantKind(t, err, apperr.KindFailedPrecondition, crm.ReasonTagLimit)
	if n := f.count("crm.customer_tags"); n != 0 {
		t.Fatalf("超過每店上限的整批仍貼了 %d 筆", n)
	}
	// 每位客戶上限：店 B，貼滿 20 個再多一個。
	f.mustOrder("evt-2", "O2", storeB, buyer1, 100)
	var names []string
	for i := 0; i < crm.MaxTagsPerCustomer; i++ {
		names = append(names, fmt.Sprintf("c%d", i))
	}
	f.mustAddTags(storeB, buyer1, names...)
	err = f.m.AddCustomerTags(ctx, storeB, buyer1, []string{"c0", "超過"})
	wantKind(t, err, apperr.KindFailedPrecondition, crm.ReasonTagLimit)
	if got := f.tagNames(storeB); len(got) != crm.MaxTagsPerCustomer {
		t.Fatalf("店 B 有 %d 個標籤，預期 %d（超過上限的整批要連新標籤一起回滾）", len(got), crm.MaxTagsPerCustomer)
	}
	// 已貼過的不算新增：貼滿後重貼同樣的 20 個仍成功。
	f.mustAddTags(storeB, buyer1, names...)
}
