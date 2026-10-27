package content

import (
	"testing"

	"google.golang.org/protobuf/proto"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/content/v1alpha1"
	contentpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/content/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/content"
)

// 對應表的每個路徑都在請求訊息裡：proto 改名時這裡會失敗，而不是讓欄位錯誤少一個路徑。
func TestFieldPathsExistInTheRequests(t *testing.T) {
	actions := []*contentpb.BannerAction{
		nil,
		{Kind: &contentpb.BannerAction_Popup_{Popup: &contentpb.BannerAction_Popup{}}},
		{Kind: &contentpb.BannerAction_Url{}},
		{Kind: &contentpb.BannerAction_ProductId{}},
		{Kind: &contentpb.BannerAction_PostId{}},
	}
	cases := []struct {
		req   proto.Message
		paths rpc.FieldPaths
	}{
		{&adminpb.CreateAppVersionGateRequest{}, gatePaths},
		{&adminpb.UpdateAppVersionGateRequest{}, gatePaths},
		{&contentpb.CheckAppVersionRequest{}, checkVersionPaths},
	}
	for _, a := range actions {
		paths := bannerPaths(&adminpb.Banner{Action: a})
		cases = append(cases,
			struct {
				req   proto.Message
				paths rpc.FieldPaths
			}{&adminpb.CreateBannerRequest{}, paths},
			struct {
				req   proto.Message
				paths rpc.FieldPaths
			}{&adminpb.UpdateBannerRequest{}, paths})
	}
	for _, c := range cases {
		desc := c.req.ProtoReflect().Descriptor()
		if missing := rpc.CheckFieldPaths(desc, c.paths); len(missing) > 0 {
			t.Errorf("%s 沒有這些路徑：%v", desc.FullName(), missing)
		}
	}
}

// 契約的列舉與模組的值域雙向一致（ADR 2026-09-25-codemeta-retirement D3 的第二道測試）。
func TestEnumsMatchTheModule(t *testing.T) {
	for _, problems := range [][]string{
		rpc.EnumMismatches[contentpb.AppType](content.AppTypes()),
	} {
		for _, p := range problems {
			t.Error(p)
		}
	}
}
