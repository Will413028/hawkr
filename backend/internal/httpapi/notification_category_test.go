package httpapi

import (
	"os"
	"strings"
	"testing"

	notificationv1 "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/notification/v1alpha1"
)

// notification 模組不能 import 這裡的生成碼；它把註冊表的分類釘成 golden（model/testdata/categories.golden），
// 這裡與 proto 的 NotificationCategory 雙向比對：多一個或少一個都會讓偏好 API 與收件匣篩選不一致。
func TestNotificationCategoryGoldenMatchesProto(t *testing.T) {
	b, err := os.ReadFile("../modules/notification/internal/model/testdata/categories.golden")
	if err != nil {
		t.Fatal(err)
	}
	fromProto := map[string]bool{}
	for num, name := range notificationv1.NotificationCategory_name {
		if num == int32(notificationv1.NotificationCategory_NOTIFICATION_CATEGORY_UNSPECIFIED) {
			continue
		}
		fromProto[strings.TrimPrefix(name, "NOTIFICATION_CATEGORY_")] = true
	}
	for _, c := range strings.Fields(string(b)) {
		if !fromProto[c] {
			t.Errorf("註冊表的分類 %s 不在 proto 的 NotificationCategory", c)
		}
		delete(fromProto, c)
	}
	for c := range fromProto {
		t.Errorf("proto 的分類 %s 不在註冊表", c)
	}
}
