//go:build integration

package seller_test

import (
	"context"
	"strings"
	"testing"

	"uuid"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
)

func TestCreateMeasureStoresProductContentAsOwnerHeld(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	product := TakedownContent{ContentRef: ContentRef{Kind: ContentProduct, ID: uuid.NewV7()}}
	post := withSnapshots([]ContentRef{{Kind: ContentPost, ID: uuid.NewV7()}})[0]
	m := f.takedownSnapshots(t, s.StoreID, []TakedownContent{product, post})

	var state string
	var bodyNull, authoredNull, imagesNull, postNull, storeNull, threadNull bool
	err := f.db.Pool().QueryRow(ctx, `SELECT snapshot_state, snapshot_body IS NULL, snapshot_authored_at IS NULL, snapshot_image_count IS NULL,
		link_post_id IS NULL, link_store_id IS NULL, link_thread_id IS NULL
		FROM seller.measure_contents WHERE measure_id = $1 AND content_kind = 'PRODUCT' AND content_id = $2`, m.ID, product.ID).
		Scan(&state, &bodyNull, &authoredNull, &imagesNull, &postNull, &storeNull, &threadNull)
	must(t, err)
	if state != "OWNER_HELD" || !bodyNull || !authoredNull || !imagesNull || !postNull || !storeNull || !threadNull {
		t.Fatalf("商品列的快照必須是 OWNER_HELD 且 snapshot_* 與 link_* 全 NULL：state=%s %v %v %v %v %v %v", state, bodyNull, authoredNull, imagesNull, postNull, storeNull, threadNull)
	}
	by := map[ContentKind]MeasureContent{}
	for _, c := range f.measureOf(t, m.ID).Contents {
		by[c.Kind] = c
	}
	if got := by[ContentProduct]; got.State != SnapshotOwnerHeld || got.Snapshot != nil || got.Excerpt != "" || got.ID != product.ID {
		t.Fatalf("讀回的商品對象 %+v", got)
	}
	if got := by[ContentPost]; got.State != SnapshotCaptured || got.Snapshot == nil {
		t.Fatalf("同一個措施的貼文仍是 CAPTURED：%+v", got)
	}
}

func TestCreateMeasureRejectsASnapshotOnAProductAndAProductTargetTwice(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	id := uuid.NewV7()
	withSnapshot := withSnapshots([]ContentRef{{Kind: ContentProduct, ID: id}})
	if _, err := f.module.CreateMeasure(ctx, f.takedownWith(s.StoreID, withSnapshot)); err == nil {
		t.Fatal("商品帶快照必須被拒絕")
	}
	f.takedownSnapshots(t, s.StoreID, []TakedownContent{{ContentRef: ContentRef{Kind: ContentProduct, ID: id}}})
	if _, err := f.module.CreateMeasure(ctx, f.takedownWith(s.StoreID, []TakedownContent{{ContentRef: ContentRef{Kind: ContentProduct, ID: id}}})); err == nil {
		t.Fatal("同一件商品已有生效中的下架措施，不能再建一個")
	}
}

func TestMeasureContentsRejectsAProductThatIsNotOwnerHeldAndOwnerHeldOnAnythingElse(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	product := TakedownContent{ContentRef: ContentRef{Kind: ContentProduct, ID: uuid.NewV7()}}
	post := withSnapshots([]ContentRef{{Kind: ContentPost, ID: uuid.NewV7()}})[0]
	m := f.takedownSnapshots(t, s.StoreID, []TakedownContent{product, post})

	_, err := f.db.Pool().Exec(ctx, `UPDATE seller.measure_contents SET snapshot_state = 'CLEARED' WHERE measure_id = $1 AND content_kind = 'PRODUCT'`, m.ID)
	if err == nil || !strings.Contains(err.Error(), "measure_contents_product_owner_held_check") {
		t.Fatalf("商品列改成 CLEARED 必須被 measure_contents_product_owner_held_check 擋下：%v", err)
	}
	_, err = f.db.Pool().Exec(ctx, `UPDATE seller.measure_contents SET snapshot_state = 'OWNER_HELD', snapshot_body = NULL, snapshot_authored_at = NULL, snapshot_image_count = NULL, link_post_id = NULL WHERE measure_id = $1 AND content_kind = 'POST'`, m.ID)
	if err == nil || !strings.Contains(err.Error(), "measure_contents_product_owner_held_check") {
		t.Fatalf("貼文列改成 OWNER_HELD 必須被 measure_contents_product_owner_held_check 擋下：%v", err)
	}
}
