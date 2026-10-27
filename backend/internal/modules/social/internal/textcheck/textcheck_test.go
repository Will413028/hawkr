package textcheck

import (
	"errors"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// 每個檢查分支的錯誤訊息來自 social/internal/text 的目錄（不是 Go 裡的字面常數），而且貼文內文與留言各用自己的稱呼。
func TestBodyMessagesComeFromCatalog(t *testing.T) {
	cases := []struct {
		name     string
		kind     Kind
		input    string
		max      int
		required bool
		want     string
	}{
		{"post invalid utf8", PostBody, "\xff", 10, false, "內文不是有效的文字"},
		{"comment invalid utf8", Comment, "\xff", 10, true, "留言不是有效的文字"},
		{"post required", PostBody, "  ", 10, true, "內文必填"},
		{"comment required", Comment, "  ", 10, true, "留言必填"},
		{"post too long", PostBody, strings.Repeat("字", 11), 10, false, "內文不可超過 10 個字"},
		{"comment too long", Comment, strings.Repeat("字", 11), 10, true, "留言不可超過 10 個字"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Body(uitext.ZhTW, "Body", tc.kind, tc.input, tc.max, tc.required)
			if apperr.KindOf(err) != apperr.KindInvalidInput {
				t.Fatalf("kind = %v, err = %v", apperr.KindOf(err), err)
			}
			var ae *apperr.Error
			if !asAppErr(err, &ae) || ae.Message != tc.want {
				t.Fatalf("message = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestBodyTrimsAndAccepts(t *testing.T) {
	got, err := Body(uitext.ZhTW, "Body", Comment, "  你好  ", 10, true)
	if err != nil || got != "你好" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func asAppErr(err error, target **apperr.Error) bool {
	return errors.As(err, target)
}
