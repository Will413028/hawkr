//go:build integration

package seller_test

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// 發布的 seller.pub_review_items(at)（step10e-3d-design.md §2.1、§6.2）：11 支 review_<種類>(at) 的 UNION ALL，規則只在那些函式。
// 這個檔的測試：集合 ⇔ 逐一套用 Go 判定式的集合、每一列等於對應函式的那一列（16 個欄位全部）、沒有多出的種類或重複的列。

// pubReviewItems 以 at 呼叫 pub_review_items，欄位同 reviewOf（seller.pub_review_item 與 review_item 同形）。
func (f *fixture) pubReviewItems(t *testing.T, at time.Time) []reviewRow {
	t.Helper()
	ctx := context.Background()
	rows, err := f.db.Conn(ctx).Query(ctx, `SELECT kind, item_id, report_id, case_id, inquiry_id, authority_order_id, target_position, account_id,
		measure_id, request_id, kyc_application_id, store_id, store_name, start_at, due_at
		FROM seller.pub_review_items($1::timestamptz)`, at)
	must(t, err)
	defer rows.Close()
	var out []reviewRow
	for rows.Next() {
		var r reviewRow
		must(t, rows.Scan(&r.Kind, &r.ItemID, &r.ReportID, &r.CaseID, &r.InquiryID, &r.OrderID, &r.Position, &r.AccountID,
			&r.MeasureID, &r.RequestID, &r.KYCID, &r.StoreID, &r.StoreName, &r.Start, &r.Due))
		out = append(out, r)
	}
	must(t, rows.Err())
	return out
}

func ptrStr[T any](p *T) string {
	if p == nil {
		return "NULL"
	}
	return fmt.Sprint(*p)
}

// rowKey 把一列的 15 個欄位全部寫成字串（時間以 UTC、ns 精度），用來整列比較。
func rowKey(r reviewRow) string {
	ts := func(t *time.Time) string {
		if t == nil {
			return "NULL"
		}
		return t.UTC().Format(time.RFC3339Nano)
	}
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s", r.Kind, r.ItemID, ptrStr(r.ReportID), ptrStr(r.CaseID), ptrStr(r.InquiryID),
		ptrStr(r.OrderID), ptrStr(r.Position), ptrStr(r.AccountID), ptrStr(r.MeasureID), ptrStr(r.RequestID), ptrStr(r.KYCID),
		ptrStr(r.StoreID), ptrStr(r.StoreName), ts(r.Start), ts(r.Due))
}

// 集合 ⇔ Go 判定式，每一列 ⇔ 對應函式的那一列：世界建完的那一刻與一週後（詢問逾期、告知、複審都進入）各比一次。
// mutation：UNION ALL 少一支函式、多一支重複的函式、或把某一支的欄位位置接錯，各自讓一個斷言失敗。
func TestPubReviewItemsIsTheUnionOfTheReviewFunctions(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := newReviewWorld(t, f)
	for _, tt := range []struct {
		name string
		at   time.Time
	}{{"建完的那一刻", w.at}, {"一週後", w.at.Add(7 * 24 * time.Hour)}} {
		f.now = tt.at
		goSets, _ := f.goReviewSets(t)
		pub := f.pubReviewItems(t, tt.at)

		pubSets := map[string]map[string]string{}
		for _, k := range reviewKinds {
			pubSets[k.kind] = map[string]string{}
		}
		for _, r := range pub {
			set, ok := pubSets[r.Kind]
			if !ok {
				t.Errorf("%s：pub_review_items 有不在 11 種裡的種類 %q（%s）", tt.name, r.Kind, r.ItemID)
				continue
			}
			if _, dup := set[r.ItemID]; dup {
				t.Errorf("%s：%s %s 在 pub_review_items 重複", tt.name, r.Kind, r.ItemID)
			}
			set[r.ItemID] = rowKey(r)
		}

		total := 0
		for _, k := range reviewKinds {
			fnRows := f.reviewSet(t, k.fn, tt.at)
			total += len(fnRows)
			// 集合 ⇔ Go 判定式。
			if len(pubSets[k.kind]) != len(goSets[k.kind]) {
				t.Errorf("%s／%s：pub_review_items %d 個，判定式 %d 個", tt.name, k.kind, len(pubSets[k.kind]), len(goSets[k.kind]))
			}
			for id := range pubSets[k.kind] {
				if !goSets[k.kind][id] {
					t.Errorf("%s／%s：%s 在 pub_review_items 裡，判定式說不是", tt.name, k.kind, id)
				}
			}
			for id := range goSets[k.kind] {
				if _, ok := pubSets[k.kind][id]; !ok {
					t.Errorf("%s／%s：%s 判定式說是，pub_review_items 裡沒有", tt.name, k.kind, id)
				}
			}
			// 每一列等於對應函式的那一列（開始、期限、店家與對象的鍵全部）。
			for id, got := range pubSets[k.kind] {
				fr, ok := fnRows[id]
				if !ok {
					t.Errorf("%s／%s：%s 在 pub_review_items 裡，%s 沒有", tt.name, k.kind, id, k.fn)
					continue
				}
				if want := rowKey(fr); got != want {
					t.Errorf("%s／%s：%s 的列與 %s 不同\n  pub: %s\n  fn:  %s", tt.name, k.kind, id, k.fn, got, want)
				}
			}
			if tt.name == "一週後" && len(pubSets[k.kind]) == 0 {
				t.Errorf("%s／%s：資料集裡沒有成員，比對是空轉", tt.name, k.kind)
			}
		}
		if len(pub) != total {
			t.Errorf("%s：pub_review_items %d 列，11 支函式合計 %d 列", tt.name, len(pub), total)
		}
		// 11 種都有期限（E3）：due_at、start_at 沒有 NULL，期限不早於開始。
		for _, r := range pub {
			if r.Start == nil || r.Due == nil || r.Due.Before(*r.Start) {
				t.Errorf("%s：%s %s 的開始 %s、期限 %s", tt.name, r.Kind, r.ItemID, fmtTime(r.Start), fmtTime(r.Due))
			}
		}
	}
}
