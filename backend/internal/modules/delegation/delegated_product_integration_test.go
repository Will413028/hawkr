//go:build integration

package delegation

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/delegatedactor"
)

// 代行商品寫入（B4-5d）：catalog 在自己的 transaction 內發帶代行者的事件，delegation 的訂閱者同 transaction 寫 SUCCEEDED（批次是一列加商品清單）；
// handler 的角色由下面的輔助函式扮演（httpapi/catalog 的 Delegated）。

var (
	productCaps = []Capability{CapabilityProductsRead, CapabilityProductsWrite, CapabilityProductsPublish}

	methodProductCreate  = DelegatedMethod{Capability: CapabilityProductsWrite, Action: AuditProductCreate, Resource: ResourceProduct}
	methodProductUpdate  = DelegatedMethod{Capability: CapabilityProductsWrite, Action: AuditProductUpdate, Resource: ResourceProduct}
	methodProductPublish = DelegatedMethod{Capability: CapabilityProductsPublish, Action: AuditProductPublish, Resource: ResourceProduct}
	methodProductUnpub   = DelegatedMethod{Capability: CapabilityProductsPublish, Action: AuditProductUnpublish, Resource: ResourceProduct}
	methodProductBatch   = DelegatedMethod{Capability: CapabilityProductsPublish, Action: AuditProductBatchUnpublish, Resource: ResourceProduct}
)

func (f *fixture) productInput(t *testing.T, title string) catalog.ProductInput {
	t.Helper()
	var leaf uuid.UUID
	if err := f.db.Pool().QueryRow(context.Background(), `SELECT id FROM catalog.categories WHERE parent_id IS NOT NULL ORDER BY id LIMIT 1`).Scan(&leaf); err != nil {
		t.Fatal(err)
	}
	return catalog.ProductInput{Title: title, CategoryID: leaf, Skus: []catalog.SkuInput{{Price: 100}}, Gallery: []catalog.Image{{Token: fmt.Sprintf("prod-tok-%d", f.next())}}}
}

func (f *fixture) uploader(storeID int64) media.Uploader {
	return media.Uploader{ID: 1, Admin: true, Store: storeID}
}

// handlerErr 扮演 handler 在 do 裡把模組錯誤轉成對外的錯誤（httpapi/catalog 的 failure）：delegation 以錯誤鏈上的原因寫 FAILED 列。
func handlerErr(err error) error {
	var batch *catalog.BatchVersionConflictError
	switch {
	case errors.As(err, &batch), errors.Is(err, catalog.ErrVersionConflict):
		return apperr.VersionConflict("商品已經被修改")
	case errors.Is(err, catalog.ErrProductNotFound):
		return apperr.NotFound("找不到這個商品")
	case errors.Is(err, catalog.ErrCreateReplayed), errors.Is(err, catalog.ErrNotPublished):
		return apperr.FailedPrecondition("%v", err)
	}
	return err
}

type wroteProduct struct {
	Product catalog.Product
	Res     DelegatedWriteResult
}

func (f *fixture) writeProduct(c DelegatedCaller, w DelegatedWrite, productID uuid.UUID,
	do func(ctx context.Context, st seller.Store, actor *delegatedactor.Actor) (catalog.Product, error)) (wroteProduct, error) {
	ctx := context.Background()
	var out wroteProduct
	res, err := f.m.DelegatedWrite(ctx, c, w, func(ctx context.Context, st seller.Store, actor *delegatedactor.Actor) error {
		p, err := do(ctx, st, actor)
		out.Product = p
		return handlerErr(err)
	})
	if err != nil {
		return wroteProduct{}, err
	}
	out.Res = res
	id := productID
	if res.Replayed {
		if id, err = uuid.Parse(res.ResourceID); err != nil {
			return wroteProduct{}, err
		}
	}
	if out.Product.ID == (uuid.UUID{}) {
		if out.Product, err = f.catalog.Product(ctx, res.Store.StoreID, id); err != nil {
			return wroteProduct{}, err
		}
	}
	return out, nil
}

func (f *fixture) createProduct(t *testing.T, c DelegatedCaller, op DelegatedOperation, in catalog.ProductInput) (wroteProduct, error) {
	t.Helper()
	return f.writeProduct(c, DelegatedWrite{Op: op, ResourceName: AuditName(in.Title), PayloadHash: DelegatedPayloadHash(AuditProductCreate, in.Title, in.Gallery[0].Token)}, uuid.UUID{},
		func(ctx context.Context, st seller.Store, actor *delegatedactor.Actor) (catalog.Product, error) {
			return f.catalog.CreateProduct(ctx, st.StoreID, f.uploader(st.StoreID), op.RequestID, in, actor)
		})
}

func (f *fixture) mustCreateProduct(t *testing.T, o operator, title string) catalog.Product {
	t.Helper()
	r, err := f.createProduct(t, f.caller(t, o, methodProductCreate), f.op("建立"), f.productInput(t, title))
	if err != nil {
		t.Fatal(err)
	}
	return r.Product
}

func (f *fixture) updateProduct(c DelegatedCaller, op DelegatedOperation, id uuid.UUID, version int64, in catalog.ProductInput) (wroteProduct, error) {
	return f.writeProduct(c, DelegatedWrite{Op: op, ResourceID: id.String(), Version: &version, PayloadHash: DelegatedPayloadHash(AuditProductUpdate, id, version, in.Title)}, id,
		func(ctx context.Context, st seller.Store, actor *delegatedactor.Actor) (catalog.Product, error) {
			return f.catalog.UpdateProduct(ctx, st.StoreID, f.uploader(st.StoreID), id, version, in, actor)
		})
}

func (f *fixture) setProductStatus(c DelegatedCaller, op DelegatedOperation, id uuid.UUID, version int64, to catalog.ProductStatus) (wroteProduct, error) {
	return f.writeProduct(c, DelegatedWrite{Op: op, ResourceID: id.String(), Version: &version, PayloadHash: DelegatedPayloadHash(c.Method.Action, id, version)}, id,
		func(ctx context.Context, st seller.Store, actor *delegatedactor.Actor) (catalog.Product, error) {
			return f.catalog.SetProductStatus(ctx, st.StoreID, id, version, to, actor)
		})
}

func (f *fixture) batchUnpublish(c DelegatedCaller, op DelegatedOperation, items []catalog.VersionedProduct) (DelegatedWriteResult, error) {
	sorted := slices.Clone(items)
	slices.SortFunc(sorted, func(a, b catalog.VersionedProduct) int { return a.ProductID.Compare(b.ProductID) })
	return f.m.DelegatedWrite(context.Background(), c, DelegatedWrite{Op: op, ResourceName: BatchResourceName(len(items)), PayloadHash: DelegatedPayloadHash(AuditProductBatchUnpublish, sorted)},
		func(ctx context.Context, st seller.Store, actor *delegatedactor.Actor) error {
			_, err := f.catalog.UnpublishProducts(ctx, st.StoreID, items, actor)
			return handlerErr(err)
		})
}

func (f *fixture) productStatus(t *testing.T, storeID int64, id uuid.UUID) (catalog.ProductStatus, int64) {
	t.Helper()
	p, err := f.catalog.Product(context.Background(), storeID, id)
	if err != nil {
		t.Fatal(err)
	}
	return p.Status, p.Version
}

func (f *fixture) activeProduct(t *testing.T, o operator, title string) catalog.Product {
	t.Helper()
	p := f.mustCreateProduct(t, o, title)
	if _, err := f.setProductStatus(f.caller(t, o, methodProductPublish), f.op("上架"), p.ID, p.Version, catalog.StatusActive); err != nil {
		t.Fatal(err)
	}
	cur, err := f.catalog.Product(context.Background(), o.storeID, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	return cur
}

func TestEveryDelegatedProductWriteLeavesOneSucceededAuditRow(t *testing.T) {
	f := build(t, true)
	ctx := context.Background()
	o := f.newOperator(t, "prod-writer", productCaps...)
	sid := o.session.Session.ID

	createOp := f.op("店家請我們代為建立")
	created, err := f.createProduct(t, f.caller(t, o, methodProductCreate), createOp, f.productInput(t, "山藥"))
	if err != nil {
		t.Fatal(err)
	}
	if created.Res.Replayed || created.Product.Status != catalog.StatusDraft || created.Product.StoreID != o.storeID {
		t.Fatalf("created = %+v", created)
	}
	updOp := f.op("修改標題")
	in := f.productInput(t, "山藥二號")
	in.Gallery = []catalog.Image{{AssetID: created.Product.Gallery[0].AssetID}}
	updated, err := f.updateProduct(f.caller(t, o, methodProductUpdate), updOp, created.Product.ID, created.Product.Version, in)
	if err != nil {
		t.Fatal(err)
	}
	pubOp := f.op("店家要求上架")
	published, err := f.setProductStatus(f.caller(t, o, methodProductPublish), pubOp, created.Product.ID, updated.Product.Version, catalog.StatusActive)
	if err != nil {
		t.Fatal(err)
	}
	unOp := f.op("內容有爭議先下架")
	unpublished, err := f.setProductStatus(f.caller(t, o, methodProductUnpub), unOp, created.Product.ID, published.Product.Version, catalog.StatusArchived)
	if err != nil {
		t.Fatal(err)
	}
	if published.Product.Status != catalog.StatusActive || unpublished.Product.Status != catalog.StatusArchived {
		t.Fatalf("published %s、unpublished %s", published.Product.Status, unpublished.Product.Status)
	}

	want := []struct {
		action, capability, reason, name string
		op                               DelegatedOperation
		result                           wroteProduct
	}{
		{"PRODUCT_CREATE", "products.write", "店家請我們代為建立", "山藥", createOp, created},
		{"PRODUCT_UPDATE", "products.write", "修改標題", "山藥二號", updOp, updated},
		{"PRODUCT_PUBLISH", "products.publish", "店家要求上架", "山藥二號", pubOp, published},
		{"PRODUCT_UNPUBLISH", "products.publish", "內容有爭議先下架", "山藥二號", unOp, unpublished},
	}
	for _, w := range want {
		if n := f.auditRows(t, sid, w.action); n != 1 {
			t.Fatalf("%s 的稽核要恰好一列：%d", w.action, n)
		}
		var (
			result, capability, resourceType, resourceID, name, key string
			reason                                                  *string
			notify, confirmed                                       bool
			after                                                   *int64
			id                                                      uuid.UUID
		)
		if err := f.db.Pool().QueryRow(ctx, `SELECT id, result, capability, resource_type, resource_id, resource_name, idempotency_key, operation_reason, notify_seller,
			confirmed_at IS NOT NULL, resource_version_after FROM delegation.admin_operation_audit_logs WHERE session_id = $1 AND action = $2`, sid, w.action).
			Scan(&id, &result, &capability, &resourceType, &resourceID, &name, &key, &reason, &notify, &confirmed, &after); err != nil {
			t.Fatal(err)
		}
		if result != "SUCCEEDED" || capability != w.capability || resourceType != "PRODUCT" || resourceID != created.Product.ID.String() || name != w.name ||
			reason == nil || *reason != w.reason || !notify || !confirmed || id != w.result.Res.AuditLogID || after == nil || *after != w.result.Product.Version || key != w.op.RequestID.String() {
			t.Fatalf("%s 的稽核列 = %s %s %s %s %q reason=%v notify=%v confirmed=%v id=%s after=%v key=%s", w.action, result, capability, resourceType, resourceID, name, reason, notify, confirmed, id, after, key)
		}
	}
	if f.resultRows(t, sid, "FAILED") != 0 {
		t.Fatalf("成功的寫入留下 %d 列 FAILED", f.resultRows(t, sid, "FAILED"))
	}
	if len(f.completed) != 4 {
		t.Fatalf("完成通知 %d 則，預期 4", len(f.completed))
	}
}

// 重送（同一個 session、同一把 request_id、同一份內容）不再呼叫 catalog：回第一次的商品與同一列稽核，不重複建立、不重複通知。
func TestDelegatedProductReplayReturnsTheFirstResultWithoutAnotherWrite(t *testing.T) {
	f := build(t, true)
	o := f.newOperator(t, "prod-replay", productCaps...)
	sid := o.session.Session.ID
	op := f.op("重送")
	in := f.productInput(t, "只該有一個")
	c := f.caller(t, o, methodProductCreate)
	first, err := f.createProduct(t, c, op, in)
	if err != nil {
		t.Fatal(err)
	}
	notified := len(f.completed)
	again, err := f.createProduct(t, c, op, in)
	if err != nil {
		t.Fatal(err)
	}
	if !again.Res.Replayed || again.Product.ID != first.Product.ID || again.Res.AuditLogID != first.Res.AuditLogID {
		t.Fatalf("重送 = %+v，預期同一個商品、同一列稽核", again)
	}
	if n := f.count(t, `SELECT count(*) FROM catalog.products WHERE store_id = $1`, o.storeID); n != 1 {
		t.Fatalf("商品 %d 個，預期 1", n)
	}
	if n := f.auditRowsOfKey(t, sid, op.RequestID.String()); n != 1 || len(f.completed) != notified {
		t.Fatalf("稽核 %d 列、通知 %d -> %d，預期 1 列、不再通知", n, notified, len(f.completed))
	}
	// 同一把 request_id、不同內容：不是重送。
	_, err = f.createProduct(t, c, op, f.productInput(t, "不同的內容"))
	if d, ok := DelegationDenialOf(err); !ok || d.Denial != DenialIdempotencyKeyReused {
		t.Fatalf("同鍵不同內容 = %v，預期 DenialIdempotencyKeyReused", err)
	}
}

func (f *fixture) batchRows(t *testing.T, sid int64) int {
	return f.auditRows(t, sid, "PRODUCT_BATCH_UNPUBLISH")
}

type batchItemRow struct {
	id            string
	before, after *int64
}

func (f *fixture) batchItemRows(t *testing.T, auditID uuid.UUID) []batchItemRow {
	t.Helper()
	rows, err := f.db.Pool().Query(context.Background(), `SELECT resource_id, resource_version_before, resource_version_after
		FROM delegation.admin_operation_audit_log_items WHERE audit_log_id = $1 ORDER BY resource_id`, auditID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []batchItemRow
	for rows.Next() {
		var r batchItemRow
		if err := rows.Scan(&r.id, &r.before, &r.after); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func versioned(ps ...catalog.Product) []catalog.VersionedProduct {
	out := make([]catalog.VersionedProduct, len(ps))
	for i, p := range ps {
		out[i] = catalog.VersionedProduct{ProductID: p.ID, Version: p.Version}
	}
	return out
}

// D29：任一項版本衝突整批不寫（商品都沒動、沒有 SUCCEEDED 列與商品清單），FAILED 列記 VERSION_CONFLICT；同一個 request_id 以正確版本重試成功，仍只有一列 SUCCEEDED。
func TestBatchUnpublishConflictWritesNothingAndTheRetryRecordsOneRow(t *testing.T) {
	f := build(t, true)
	o := f.newOperator(t, "batch-conflict", productCaps...)
	sid := o.session.Session.ID
	a, b, c3 := f.activeProduct(t, o, "甲"), f.activeProduct(t, o, "乙"), f.activeProduct(t, o, "丙")
	completed := len(f.completed)
	call := f.caller(t, o, methodProductBatch)
	op := f.op("批次下架")

	stale := versioned(a, b, c3)
	stale[1].Version += 100
	_, err := f.batchUnpublish(call, op, stale)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Reason != apperr.ReasonVersionConflict {
		t.Fatalf("過期的批次 = %v，預期 VERSION_CONFLICT", err)
	}
	for _, p := range []catalog.Product{a, b, c3} {
		if st, v := f.productStatus(t, o.storeID, p.ID); st != catalog.StatusActive || v != p.Version {
			t.Fatalf("商品 %s 被拒絕的批次改動：%s v%d", p.ID, st, v)
		}
	}
	if f.resultRows(t, sid, "SUCCEEDED") != 3 /* 三個上架 */ +3 /* 三個建立 */ || f.batchRows(t, sid) != 0 {
		t.Fatalf("被拒絕的批次留下 SUCCEEDED 的批次列")
	}
	row := f.failedRow(t, sid, "PRODUCT_BATCH_UNPUBLISH")
	if row.n != 1 || row.failureReason == nil || *row.failureReason != "VERSION_CONFLICT" || row.key != nil || row.capability != "products.publish" || row.notify {
		t.Fatalf("FAILED 列 = %+v", row)
	}
	if len(f.completed) != completed {
		t.Fatalf("被拒絕的批次通知了賣家：%d -> %d", completed, len(f.completed))
	}
	if _, err := f.batchUnpublish(call, op, versioned(a, b, c3)); err != nil {
		t.Fatal(err)
	}
	if n := f.auditRowsOfKey(t, sid, op.RequestID.String()); n != 1 || f.batchRows(t, sid) != 1 {
		t.Fatalf("重試後冪等鍵 %d 列、批次列 %d，預期各 1", n, f.batchRows(t, sid))
	}
}

// D29：一個 request_id、一個 transaction、一列稽核（記錄本批商品與版本）、一則賣家通知；已下架的項目視為成功（版本不變）；同一個 request_id 重送回原結果、不再寫稽核與通知。
func TestBatchUnpublishWritesOneAuditRowWithItsItemsAndOneNotification(t *testing.T) {
	f := build(t, true)
	ctx := context.Background()
	o := f.newOperator(t, "batch-ok", productCaps...)
	sid := o.session.Session.ID
	a, b := f.activeProduct(t, o, "甲"), f.activeProduct(t, o, "乙")
	already := f.activeProduct(t, o, "已下架")
	if _, err := f.setProductStatus(f.caller(t, o, methodProductUnpub), f.op("先下架"), already.ID, already.Version, catalog.StatusArchived); err != nil {
		t.Fatal(err)
	}
	already, _ = f.catalog.Product(ctx, o.storeID, already.ID)
	completed := len(f.completed)
	succeeded := f.resultRows(t, sid, "SUCCEEDED")
	call := f.caller(t, o, methodProductBatch)
	op := f.op("店家要求全部下架")
	items := versioned(a, b, already)

	res, err := f.batchUnpublish(call, op, items)
	if err != nil {
		t.Fatal(err)
	}
	if res.Replayed || res.ResourceID != "" {
		t.Fatalf("結果 = %+v，批次的稽核列沒有單一資源", res)
	}
	for _, p := range []catalog.Product{a, b, already} {
		if st, _ := f.productStatus(t, o.storeID, p.ID); st != catalog.StatusArchived {
			t.Fatalf("商品 %s = %s，預期 ARCHIVED", p.ID, st)
		}
	}
	if _, v := f.productStatus(t, o.storeID, already.ID); v != already.Version {
		t.Fatalf("已下架的項目版本 %d，預期不變的 %d", v, already.Version)
	}
	if f.resultRows(t, sid, "SUCCEEDED") != succeeded+1 || f.batchRows(t, sid) != 1 {
		t.Fatalf("批次要恰好一列 SUCCEEDED：總數 %d -> %d、批次列 %d", succeeded, f.resultRows(t, sid, "SUCCEEDED"), f.batchRows(t, sid))
	}
	if len(f.completed) != completed+1 {
		t.Fatalf("賣家通知 %d -> %d，預期恰好多一則", completed, len(f.completed))
	}
	var (
		resourceID *string
		name       string
		reason     *string
		notify     bool
	)
	if err := f.db.Pool().QueryRow(ctx, `SELECT resource_id, resource_name, operation_reason, notify_seller FROM delegation.admin_operation_audit_logs WHERE id = $1`, res.AuditLogID).
		Scan(&resourceID, &name, &reason, &notify); err != nil {
		t.Fatal(err)
	}
	if resourceID != nil || name != "3 項商品" || reason == nil || *reason != "店家要求全部下架" || !notify {
		t.Fatalf("批次列 resource_id=%v name=%q reason=%v notify=%v", resourceID, name, reason, notify)
	}
	rows := f.batchItemRows(t, res.AuditLogID)
	ids := []uuid.UUID{a.ID, b.ID, already.ID}
	slices.SortFunc(ids, uuid.UUID.Compare)
	if len(rows) != 3 {
		t.Fatalf("商品清單 %d 項，預期 3", len(rows))
	}
	for i, r := range rows {
		if r.id != ids[i].String() || r.before == nil || r.after == nil {
			t.Fatalf("清單第 %d 項 = %+v，預期 %s 且帶版本", i, r, ids[i])
		}
		if r.id == already.ID.String() {
			if *r.before != already.Version || *r.after != already.Version {
				t.Fatalf("已下架項目的版本 %d -> %d，預期都是 %d", *r.before, *r.after, already.Version)
			}
		} else if *r.after != *r.before+1 {
			t.Fatalf("下架項目的版本 %d -> %d，預期加一", *r.before, *r.after)
		}
	}

	again, err := f.batchUnpublish(call, op, items)
	if err != nil {
		t.Fatal(err)
	}
	if !again.Replayed || again.AuditLogID != res.AuditLogID || len(again.Items) != 3 {
		t.Fatalf("重送 = %+v，預期回第一次的稽核列與三項商品", again)
	}
	if f.batchRows(t, sid) != 1 || len(f.completed) != completed+1 || len(f.batchItemRows(t, res.AuditLogID)) != 3 {
		t.Fatal("重送不該再寫稽核、商品清單或通知")
	}
	// 同一把 request_id、不同的商品：不是重送。
	_, err = f.batchUnpublish(call, op, versioned(a))
	if d, ok := DelegationDenialOf(err); !ok || d.Denial != DenialIdempotencyKeyReused {
		t.Fatalf("同鍵不同內容 = %v，預期 DenialIdempotencyKeyReused", err)
	}
}

// 批次裡有別家的商品：整批 not found、什麼都不動，FAILED 列記 NOT_FOUND。
func TestBatchUnpublishWithAnotherStoresProductWritesNothing(t *testing.T) {
	f := build(t, true)
	o := f.newOperator(t, "batch-store-a", productCaps...)
	other := f.newOperator(t, "batch-store-b", productCaps...)
	mine, theirs := f.activeProduct(t, o, "我的"), f.activeProduct(t, other, "別家的")
	sid := o.session.Session.ID

	_, err := f.batchUnpublish(f.caller(t, o, methodProductBatch), f.op("越界"), versioned(mine, theirs))
	var ae *apperr.Error
	if !errors.As(err, &ae) || apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("混入別家商品的批次 = %v，預期 not found", err)
	}
	if st, _ := f.productStatus(t, o.storeID, mine.ID); st != catalog.StatusActive {
		t.Fatalf("我的商品 = %s，整批沒寫入", st)
	}
	if st, _ := f.productStatus(t, other.storeID, theirs.ID); st != catalog.StatusActive {
		t.Fatalf("別家的商品 = %s，不該被動到", st)
	}
	row := f.failedRow(t, sid, "PRODUCT_BATCH_UNPUBLISH")
	if row.n != 1 {
		t.Fatalf("FAILED 列 %d，預期 1", row.n)
	}
	wantFailureReason(t, "別家商品的批次", row.failureReason, "NOT_FOUND")
	if f.batchRows(t, sid) != 0 {
		t.Fatal("不該有 SUCCEEDED 的批次列")
	}
}

// 代行寫入只在 session 的店家：用別家的商品 ID 修改、上下架都是 not found，別家的商品不動，留下 FAILED。
func TestDelegatedProductWritesStayInTheSessionStore(t *testing.T) {
	f := build(t, true)
	o := f.newOperator(t, "scope-a", productCaps...)
	other := f.newOperator(t, "scope-b", productCaps...)
	theirs := f.activeProduct(t, other, "別家的")
	sid := o.session.Session.ID

	_, err := f.updateProduct(f.caller(t, o, methodProductUpdate), f.op("越界修改"), theirs.ID, theirs.Version, f.productInput(t, "被改了"))
	if apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("修改別家商品 = %v，預期 not found", err)
	}
	_, err = f.setProductStatus(f.caller(t, o, methodProductUnpub), f.op("越界下架"), theirs.ID, theirs.Version, catalog.StatusArchived)
	if apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("下架別家商品 = %v，預期 not found", err)
	}
	p, err := f.catalog.Product(context.Background(), other.storeID, theirs.ID)
	if err != nil || p.Status != catalog.StatusActive || p.Title != "別家的" || p.Version != theirs.Version {
		t.Fatalf("別家的商品被動到：%+v %v", p, err)
	}
	for _, a := range []string{"PRODUCT_UPDATE", "PRODUCT_UNPUBLISH"} {
		wantFailureReason(t, a, f.failedRow(t, sid, a).failureReason, "NOT_FOUND")
	}
}

// 缺少能力的 session：每一種商品寫入都被擋在 catalog 之前（沒有商品被建立或改動），留下 FAILED（CAPABILITY_DENIED）；有 products.read 的讀取不稽核。
func TestMissingProductCapabilityIsRejectedForEveryWriteAndLeavesFailedRows(t *testing.T) {
	f := build(t, true)
	full := f.newOperator(t, "prod-full", productCaps...)
	p := f.activeProduct(t, full, "既有商品")
	o := f.newOperator(t, "prod-readonly", CapabilityProductsRead)
	// 同一家店的唯讀 session：換成唯讀的店家授權不方便，直接用唯讀 operator 的 session 對自己的店寫（沒有商品）與對 full 的商品寫（別家）都要被擋在能力檢查。
	sid := o.session.Session.ID
	deny := func(name string, err error) {
		t.Helper()
		if d, ok := DelegationDenialOf(err); !ok || d.Denial != DenialCapabilityDenied {
			t.Errorf("%s：%v，預期 DenialCapabilityDenied", name, err)
		}
	}
	_, err := f.createProduct(t, f.caller(t, o, methodProductCreate), f.op("r"), f.productInput(t, "x"))
	deny("建立", err)
	_, err = f.updateProduct(f.caller(t, o, methodProductUpdate), f.op("r"), p.ID, p.Version, f.productInput(t, "x"))
	deny("修改", err)
	_, err = f.setProductStatus(f.caller(t, o, methodProductPublish), f.op("r"), p.ID, p.Version, catalog.StatusActive)
	deny("上架", err)
	_, err = f.setProductStatus(f.caller(t, o, methodProductUnpub), f.op("r"), p.ID, p.Version, catalog.StatusArchived)
	deny("下架", err)
	_, err = f.batchUnpublish(f.caller(t, o, methodProductBatch), f.op("r"), versioned(p))
	deny("批次下架", err)
	if n := f.resultRows(t, sid, "SUCCEEDED"); n != 0 {
		t.Fatalf("被擋下的寫入留下 %d 列 SUCCEEDED", n)
	}
	for _, a := range []string{"PRODUCT_CREATE", "PRODUCT_UPDATE", "PRODUCT_PUBLISH", "PRODUCT_UNPUBLISH", "PRODUCT_BATCH_UNPUBLISH"} {
		row := f.failedRow(t, sid, a)
		if row.n != 1 {
			t.Fatalf("%s 的 FAILED %d 列，預期 1", a, row.n)
		}
		wantFailureReason(t, a, row.failureReason, "CAPABILITY_DENIED")
	}
	if n := f.count(t, `SELECT count(*) FROM catalog.products WHERE store_id = $1`, o.storeID); n != 0 {
		t.Fatalf("被擋下的建立仍寫了 %d 個商品", n)
	}
	if st, v := f.productStatus(t, full.storeID, p.ID); st != catalog.StatusActive || v != p.Version {
		t.Fatalf("既有商品被動到：%s v%d", st, v)
	}
	// 只有 products.write 的 session 不能上下架（products.publish 是獨立的能力）。
	w := f.newOperator(t, "prod-writeonly", CapabilityProductsWrite)
	mine := f.mustCreateProduct(t, w, "自己的")
	_, err = f.setProductStatus(f.caller(t, w, methodProductPublish), f.op("r"), mine.ID, mine.Version, catalog.StatusActive)
	deny("只有 write 的上架", err)
	if st, _ := f.productStatus(t, w.storeID, mine.ID); st != catalog.StatusDraft {
		t.Fatalf("商品 = %s，預期仍是草稿", st)
	}
}

// 店主自己的商品寫入（沒有代行者）不留代行稽核列，批次下架也一樣。
func TestOwnerProductWritesLeaveNoAuditRows(t *testing.T) {
	f := build(t, true)
	ctx := context.Background()
	o := f.newOperator(t, "prod-owner", productCaps...)
	p, err := f.catalog.CreateProduct(ctx, o.storeID, f.uploader(o.storeID), uuid.NewV7(), f.productInput(t, "店主自己建的"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.catalog.SetProductStatus(ctx, o.storeID, p.ID, p.Version, catalog.StatusActive, nil); err != nil {
		t.Fatal(err)
	}
	cur, _ := f.catalog.Product(ctx, o.storeID, p.ID)
	if _, err := f.catalog.UnpublishProducts(ctx, o.storeID, versioned(cur), nil); err != nil {
		t.Fatal(err)
	}
	if n := f.count(t, `SELECT count(*) FROM delegation.admin_operation_audit_logs WHERE target_store_id = $1`, o.storeID); n != 0 {
		t.Fatalf("店主的寫入留下 %d 列代行稽核", n)
	}
}

// 並行的同 request_id（同 session、同內容）：輸家的寫入可能因贏家已提交而版本衝突，不能寫 FAILED、不能回 ABORTED，兩個呼叫都回第一次的結果，稽核只有一列。
func TestConcurrentWritesWithTheSameRequestIDAllReturnTheFirstResultAndRecordOneRow(t *testing.T) {
	f := build(t, true)
	o := f.newOperator(t, "prod-concurrent", productCaps...)
	sid := o.session.Session.ID
	p := f.mustCreateProduct(t, o, "山藥")
	call := f.caller(t, o, methodProductUpdate)
	op := f.op("並行修改標題")
	const n = 4
	inputs := make([]catalog.ProductInput, n)
	for i := range inputs {
		inputs[i] = f.productInput(t, "山藥二號")
		inputs[i].Gallery = []catalog.Image{{AssetID: p.Gallery[0].AssetID}}
	}
	type outcome struct {
		res wroteProduct
		err error
	}
	got := make([]outcome, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			got[i].res, got[i].err = f.updateProduct(call, op, p.ID, p.Version, inputs[i])
		}()
	}
	close(start)
	wg.Wait()
	var first uuid.UUID
	for i, g := range got {
		if g.err != nil {
			t.Fatalf("呼叫 %d = %v，並行的同 request_id 都要成功", i, g.err)
		}
		if i == 0 {
			first = g.res.Res.AuditLogID
		}
		if g.res.Res.AuditLogID != first || g.res.Product.ID != p.ID {
			t.Fatalf("呼叫 %d 的稽核列 %s、商品 %s，預期同一列 %s、商品 %s", i, g.res.Res.AuditLogID, g.res.Product.ID, first, p.ID)
		}
	}
	if n := f.auditRowsOfKey(t, sid, op.RequestID.String()); n != 1 {
		t.Fatalf("冪等鍵 %d 列，預期 1", n)
	}
	if r := f.failedRow(t, sid, "PRODUCT_UPDATE"); r.n != 0 {
		t.Fatalf("並行的重送留下 %d 列 FAILED（%v）", r.n, r.failureReason)
	}
	if _, v := f.productStatus(t, o.storeID, p.ID); v != p.Version+1 {
		t.Fatalf("商品版本 %d，預期只改一次的 %d", v, p.Version+1)
	}
}

// 代行建立的 request_id 已被這家店的另一次建立用過（不是這個 session 的重送）：FAILED_PRECONDITION，不是 INTERNAL；沒有 SUCCEEDED 的建立列、沒有第二個商品。
func TestDelegatedCreateWithARequestIDTheStoreAlreadyUsedFailsAPreconditionNotInternal(t *testing.T) {
	f := build(t, true)
	ctx := context.Background()
	o := f.newOperator(t, "prod-used-rid", productCaps...)
	sid := o.session.Session.ID
	op := f.op("建立")
	in := f.productInput(t, "店主先建的")
	if _, err := f.catalog.CreateProduct(ctx, o.storeID, f.uploader(o.storeID), op.RequestID, in, nil); err != nil {
		t.Fatal(err)
	}
	_, err := f.createProduct(t, f.caller(t, o, methodProductCreate), op, in)
	if apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("建立 = %v，預期 FAILED_PRECONDITION", err)
	}
	row := f.failedRow(t, sid, "PRODUCT_CREATE")
	if row.n != 1 || row.failureReason == nil || *row.failureReason != "FAILED_PRECONDITION" {
		t.Fatalf("FAILED 列 = %+v，預期原因 FAILED_PRECONDITION", row)
	}
	if f.auditRows(t, sid, "PRODUCT_CREATE") != 1 || f.resultRows(t, sid, "SUCCEEDED") != 0 {
		t.Fatal("被拒絕的建立不該留下 SUCCEEDED 列")
	}
	if n := f.count(t, `SELECT count(*) FROM catalog.products WHERE store_id = $1`, o.storeID); n != 1 {
		t.Fatalf("商品 %d 個，預期 1", n)
	}
}

// 批次裡有草稿：整批 FAILED_PRECONDITION、沒有任何商品被動、沒有 SUCCEEDED 的批次列。
func TestBatchUnpublishWithADraftWritesNothing(t *testing.T) {
	f := build(t, true)
	o := f.newOperator(t, "batch-draft", productCaps...)
	sid := o.session.Session.ID
	active := f.activeProduct(t, o, "上架中")
	draft := f.mustCreateProduct(t, o, "草稿")
	_, err := f.batchUnpublish(f.caller(t, o, methodProductBatch), f.op("含草稿"), versioned(active, draft))
	if apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("含草稿的批次 = %v，預期 FAILED_PRECONDITION", err)
	}
	if st, _ := f.productStatus(t, o.storeID, active.ID); st != catalog.StatusActive {
		t.Fatalf("上架中的商品 = %s，整批沒寫入", st)
	}
	if st, _ := f.productStatus(t, o.storeID, draft.ID); st != catalog.StatusDraft {
		t.Fatalf("草稿 = %s，預期仍是 DRAFT", st)
	}
	if f.batchRows(t, sid) != 1 || f.failedRow(t, sid, "PRODUCT_BATCH_UNPUBLISH").n != 1 {
		t.Fatal("預期只有一列 FAILED 的批次稽核")
	}
}
