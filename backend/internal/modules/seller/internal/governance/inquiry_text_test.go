package governance

import (
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// 詢問不存在的句子來自共用目錄的 common.error.NOT_FOUND：key 不見了或句子是空的，這個測試失敗，而不是執行期回出空白訊息。
func TestInquiryNotFoundTextIsInTheCatalog(t *testing.T) {
	want, ok := uitext.For(uitext.ZhTW).CommonError("NOT_FOUND")
	if !ok || want.Text == "" || want.Locale != uitext.ZhTW {
		t.Fatalf("目錄缺 common.error.NOT_FOUND 的 zh-TW 句子：%+v ok=%v", want, ok)
	}
	err := errInquiryNotFound()
	if apperr.KindOf(err) != apperr.KindNotFound || !strings.Contains(err.Error(), want.Text) {
		t.Fatalf("err=%v，預期 not_found 與句子 %q", err, want.Text)
	}
}
