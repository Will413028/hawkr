package catalog

import (
	"unicode/utf8"

	"google.golang.org/protobuf/proto"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/catalog/v1alpha1"
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 請求層的內容上限：值取自 proto 欄位上的 hawkr.common.v1alpha1.limits（唯一來源），不在這裡寫第二份數字。
// 寫入內容（ProductInput）的上限由模組驗證；這裡只有模組不看的請求欄位（篩選與關鍵字）。
var (
	maxFilterCategories = limitOf(&pb.ProductFilter{}, "category_ids").GetMaxItems()
	maxFilterHashtags   = limitOf(&pb.ProductFilter{}, "hashtags").GetMaxItems()
	maxFilterStores     = limitOf(&pb.ProductFilter{}, "store_ids").GetMaxItems()
	maxSearchRunes      = limitOf(&pb.SearchProductsRequest{}, "query").GetMaxRunes()
	maxSuggestRunes     = limitOf(&pb.SuggestProductNamesRequest{}, "query").GetMaxRunes()
	maxMyQueryRunes     = limitOf(&pb.MyProductFilter{}, "query").GetMaxRunes()
)

func limitOf(m proto.Message, field string) *commonpb.Limits {
	f := m.ProtoReflect().Descriptor().Fields().ByName(protoreflectName(field))
	if f == nil {
		panic("httpapi/catalog：找不到欄位 " + field)
	}
	l, _ := proto.GetExtension(f.Options(), commonpb.E_Limits).(*commonpb.Limits)
	if l == nil {
		panic("httpapi/catalog：欄位 " + field + " 沒有宣告 limits")
	}
	return l
}

// tooMany 是 repeated 欄位 path 超過 max 個元素的錯誤。
func tooMany(path string, n int, max int32) error {
	if int32(n) <= max { //nolint:gosec // n 是請求裡 repeated 欄位的長度，遠小於 int32
		return nil
	}
	return fieldError(path, "最多 %d 個", max)
}

// tooLong 是字串欄位 path 超過 max 個字元（code point）的錯誤。
func tooLong(path, s string, max int32) error {
	if int32(utf8.RuneCountInString(s)) <= max { //nolint:gosec // 請求大小有上限（maxRequestBytes）
		return nil
	}
	return fieldError(path, "最多 %d 個字元", max)
}

// fieldError 是指向請求欄位 path 的 invalid_argument。
func fieldError(path, format string, args ...any) error {
	return withPaths(apperr.InvalidInputOn([]string{path}, format, args...), path)
}
