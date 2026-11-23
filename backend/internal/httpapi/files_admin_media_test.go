package httpapi

import (
	"testing"

	"github.com/Will413028/hawkr/backend/internal/httpapi/adminauth"
	httpmedia "github.com/Will413028/hawkr/backend/internal/httpapi/media"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
)

// 代行的段上傳與預覽路由（B1-D16）不是後台路由：憑證只有代行 session，所以掛在 /api/files/delegated/ 底下，不宣告管理員權限，
// 也不會被 GuardFile 當成要管理員 Bearer 的路由；路由表要有這兩條，而且舊的 /api/files/admin/secure_uploads 不再存在。
func TestDelegatedSecureUploadRoutesAreNotAdminRoutes(t *testing.T) {
	routes := fileRoutes(Deps{Media: &media.Module{}})
	found := map[string]bool{}
	for _, r := range routes {
		switch r.pattern {
		case httpmedia.DelegatedChunkRoute, httpmedia.DelegatedPreviewRoute:
			found[r.pattern] = true
			if r.permission != "" {
				t.Errorf("%s 不得宣告管理員權限，卻有 %q", r.pattern, r.permission)
			}
			if adminauth.IsAdminFileRoute(r.pattern) {
				t.Errorf("%s 被當成後台檔案路由", r.pattern)
			}
		}
		if _, err := adminauth.GuardFile(r.pattern, r.permission, r.handler, nil); err != nil {
			t.Errorf("路由 %s 的宣告不合法：%v", r.pattern, err)
		}
	}
	if len(found) != 2 {
		t.Fatalf("檔案路由表缺代行路由，找到 %v", found)
	}
}
