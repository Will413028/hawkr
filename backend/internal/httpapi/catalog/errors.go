package catalog

import (
	"errors"
	"regexp"
	"strings"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 模組回報的欄位名（Go 風格，索引寫成 [n]）對到請求的 proto 路徑；[] 依序填入模組回報的索引（rpc.FieldPaths）。
// 寫入商品的請求把 ProductInput 放在 product。

// inputPaths 是 ProductInput 內的欄位，prefix 是 ProductInput 在請求裡的路徑（"product."）。
func inputPaths(prefix string) rpc.FieldPaths {
	p := map[string]string{
		"Title":                       "title",
		"CategoryID":                  "category_id",
		"LowStockThreshold":           "low_stock_threshold",
		"Hashtags":                    "hashtag_inputs",
		"Gallery":                     "gallery",
		"Gallery[]":                   "gallery[]",
		"Gallery[].AssetID":           "gallery[].asset_id",
		"Gallery[].Alt":               "gallery[].alt",
		"Axes":                        "axes",
		"Axes[]":                      "axes[]",
		"Axes[].Name":                 "axes[].name",
		"Axes[].Values":               "axes[].values",
		"Axes[].Values[]":             "axes[].values[]",
		"Skus":                        "skus",
		"Skus[]":                      "skus[]",
		"Skus[].Option1":              "skus[].option1",
		"Skus[].Option2":              "skus[].option2",
		"Skus[].Price":                "skus[].price",
		"Skus[].CompareAtPrice":       "skus[].compare_at_price",
		"Skus[].StockQty":             "skus[].initial_stock",
		"Description":                 "description",
		"Description[]":               "description[]",
		"Description[].Kind":          "description[]",
		"Description[].Text":          "description[].paragraph",
		"Description[].Image":         "description[].image",
		"Description[].Image.AssetID": "description[].image.asset_id",
		"Description[].Image.Alt":     "description[].image.alt",
		"Description[].Items":         "description[].bullet_list.items",
		"Description[].Items[]":       "description[].bullet_list.items[]",
		"Description[].Rows":          "description[].table.rows",
		"Description[].Rows[]":        "description[].table.rows[]",
		"Description[].Rows[][]":      "description[].table.rows[].cells[]",
		"Description.Image":           "description",
	}
	out := make(rpc.FieldPaths, len(p))
	for k, v := range p {
		out[k] = prefix + v
	}
	return out
}

var (
	createPaths = merge(inputPaths("product."), rpc.FieldPaths{"RequestID": "request_id"})
	updatePaths = merge(inputPaths("product."), rpc.FieldPaths{"Etag": "etag"})
	statusPaths = rpc.FieldPaths{"Status": "status", "Etag": "etag"}
	deletePaths = rpc.FieldPaths{"Etag": "etag"}
	adjustPaths = rpc.FieldPaths{"RequestID": "request_id", "Delta": "delta"}
	setPaths    = rpc.FieldPaths{"RequestID": "request_id", "Quantity": "quantity"}
	// 列表與計數的篩選：賣家的在 filter，公開的 Sort 與標籤在請求上。
	sellerListPaths = rpc.FieldPaths{"Status": "filter.status", "StockStatus": "filter.stock_status"}
	publicListPaths = rpc.FieldPaths{"Sort": "sort", "Hashtags": "filter.hashtags"}
	noPaths         = rpc.FieldPaths{}
)

func merge(a, b rpc.FieldPaths) rpc.FieldPaths {
	out := make(rpc.FieldPaths, len(a)+len(b))
	for _, m := range []rpc.FieldPaths{a, b} {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

// imageRefIndex 是「圖庫與描述裡同一張圖出現兩次」的欄位 Description.Image[n]：n 是描述裡第幾張圖，不是第幾個區塊，沒有對得上的請求索引，
// 所以指向整個 description。
var imageRefIndex = regexp.MustCompile(`^Description\.Image\[\d+\]$`)

// reasonText 是模組回報的欄位原因（catalog.Reason*）的使用者面文字，整句是「<欄位>」之後接的描述。
var reasonText = map[string]string{
	catalog.ReasonRequired:   "必填",
	catalog.ReasonTooLong:    "太長",
	catalog.ReasonTooMany:    "數量超過上限",
	catalog.ReasonDuplicate:  "重複",
	catalog.ReasonUnknown:    "不在可選的範圍內",
	catalog.ReasonOutOfRange: "超出允許的範圍",
	catalog.ReasonInvalid:    "格式不正確",
}

// failure 把 catalog 模組的錯誤轉成對外的錯誤；paths 是這個方法的欄位對應。沒有分類的錯誤原樣回傳（interceptor 記為 internal）。
// 商品不存在與商品不是這家店的都回 not_found（防枚舉，規範 §二）。
func failure(err error, paths rpc.FieldPaths) error {
	if err == nil {
		return nil
	}
	var invalid *catalog.InvalidInputError
	var shortage *catalog.InsufficientStockError
	var mismatch *catalog.StockMismatchError
	var batchConflict *catalog.BatchVersionConflictError
	var batchDrafts *catalog.BatchNotPublishedError
	switch {
	case errors.As(err, &invalid):
		field := invalid.Field
		if imageRefIndex.MatchString(field) {
			field = "Description.Image"
		}
		text := reasonText[invalid.Reason]
		if text == "" {
			text = "不合法"
		}
		return rpc.WithFieldPaths(apperr.InvalidInputOn([]string{field}, "%s", fieldLabel(field)+text), paths)
	case errors.Is(err, catalog.ErrProductNotFound):
		return apperr.NotFound("找不到這個商品")
	case errors.Is(err, catalog.ErrSkuNotFound):
		return apperr.NotFound("找不到這個規格")
	case errors.As(err, &batchConflict):
		ids := batchConflict.SortedIDs()
		text := make([]string, len(ids))
		for i, id := range ids {
			text[i] = id.String()
		}
		return apperr.VersionConflict("有 %d 個商品已經被修改，請重新讀取後再送出", len(ids)).WithMetadata("conflictingProductIds", strings.Join(text, ","))
	case errors.As(err, &batchDrafts):
		text := make([]string, len(batchDrafts.ProductIDs))
		for i, id := range batchDrafts.ProductIDs {
			text[i] = id.String()
		}
		return apperr.FailedPrecondition("有 %d 個商品是草稿，草稿沒有上架過、不能下架", len(text)).
			WithReason(apperr.ReasonProductNotPublished).WithMetadata("notPublishedProductIds", strings.Join(text, ","))
	case errors.Is(err, catalog.ErrNotPublished):
		return apperr.FailedPrecondition("草稿沒有上架過，不能下架").WithReason(apperr.ReasonProductNotPublished)
	case errors.Is(err, catalog.ErrCreateReplayed):
		return apperr.FailedPrecondition("這個 request_id 已經用來建立過這家店的商品，請改用新的 request_id")
	case errors.Is(err, catalog.ErrVersionConflict):
		return apperr.VersionConflict("商品已經被修改，請重新讀取後再送出")
	case errors.Is(err, catalog.ErrActiveRequiresGalleryImage):
		return apperr.FailedPrecondition("上架中的商品至少要有一張主圖")
	case errors.Is(err, catalog.ErrIdempotencyKeyReused):
		return rpc.WithFieldPaths(apperr.InvalidInputOn([]string{"RequestID"}, "這個 request_id 已經用在內容不同的請求"), paths)
	case errors.As(err, &shortage):
		return apperr.FailedPrecondition("調整後的庫存不得小於 0")
	case errors.As(err, &mismatch):
		return apperr.FailedPrecondition("庫存已經變動（現在是 %d），請重新讀取後再設定", mismatch.Current)
	case errors.Is(err, catalog.ErrInvalidStockInput):
		return apperr.InvalidInput("庫存的數量不合法")
	}
	return err
}

// fieldLabel 是欄位給使用者看的名稱前綴；請求路徑已經指出是哪個欄位，這裡只給一個可讀的名字，不含索引。
func fieldLabel(field string) string {
	name, _, _ := strings.Cut(field, "[")
	name, _, _ = strings.Cut(name, ".")
	switch name {
	case "Title":
		return "商品標題"
	case "CategoryID":
		return "分類"
	case "Hashtags":
		return "標籤"
	case "Gallery":
		return "主圖"
	case "Description":
		return "描述"
	case "Axes":
		return "選項軸"
	case "Skus":
		return "規格"
	case "LowStockThreshold":
		return "低庫存門檻"
	case "Etag":
		return "etag"
	case "Status", "StockStatus":
		return "狀態"
	case "Sort":
		return "排序"
	}
	return "欄位"
}
