package governance

import (
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// 賣家端授權的唯一判斷：店家所屬的帳號等於呼叫的帳號；0 與負的帳號永遠不擁有。
func TestOwnsInquiryOnlyForTheStoresAccount(t *testing.T) {
	for name, c := range map[string]struct {
		storeAccount, caller int64
		want                 bool
	}{
		"同一個帳號":   {7, 7, true},
		"別的帳號":    {7, 8, false},
		"帳號是 0":   {0, 0, false},
		"帳號是負的":   {-1, -1, false},
		"店家帳號為 0": {0, 7, false},
	} {
		if got := ownsInquiry(c.storeAccount, c.caller); got != c.want {
			t.Errorf("%s：got=%v", name, got)
		}
	}
}

// 賣家回覆被擋下的句子在兩個語系都有字，且 Availability 的每個原因都有對應的句子；沒有原因不是錯誤。
func TestSellerReplyBlockTextCoversEveryBlockInBothLocales(t *testing.T) {
	for _, b := range model.InquiryBlockValues() {
		for _, l := range []uitext.Locale{uitext.ZhTW, uitext.EN} {
			if sellerInquiryBlockText(l, b).Text == "" {
				t.Errorf("%s 在 %s 沒有句子", b, l)
			}
		}
	}
	if sellerInquiryBlockText(uitext.EN, model.InquiryBlockReplied).Text == sellerInquiryBlockText(uitext.ZhTW, model.InquiryBlockReplied).Text {
		t.Error("en 與 zh-TW 的句子相同，en 沒有翻譯")
	}
	if err := sellerReplyBlocked(uitext.ZhTW, nil); err != nil {
		t.Fatalf("沒有原因：%v", err)
	}
	err := sellerReplyBlocked(uitext.ZhTW, model.CaseInquiry{Withdrawn: &model.InquiryWithdrawal{}}.Availability().RecordReply)
	if apperr.KindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), "已撤回") {
		t.Fatalf("err=%v", err)
	}
	err = sellerReplyBlocked(uitext.ZhTW, model.CaseInquiry{Reply: &model.InquiryReply{}}.Availability().RecordReply)
	if apperr.KindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), "已經回覆過") {
		t.Fatalf("err=%v", err)
	}
}

// 逾期不擋回覆：Availability 沒有逾期這個原因，只看撤回與已回覆。
func TestOverdueDoesNotBlockTheReply(t *testing.T) {
	if blocks := (model.CaseInquiry{}).Availability().RecordReply; len(blocks) != 0 {
		t.Fatalf("blocks=%v", blocks)
	}
}
