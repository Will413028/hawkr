package seller

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/type/date"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// LookupBusinessRegistration 以統編查公開的登記資料，經模組（登記資料來源、現況分類與地址拆分都在模組，與 KYC 送件的查詢快照同一個
// 入口）。外部呼叫在模組、不在任何 transaction 內；查無是回應沒有 registration，資料來源沒有給出可用的答案是 unavailable（兩者不混為
// 一談，畫面才能分辨「沒有這家公司」與「稍後再試」）。限流與格式檢查在這裡與入口。
func (h *App) LookupBusinessRegistration(ctx context.Context, req *connect.Request[pb.LookupBusinessRegistrationRequest]) (*connect.Response[pb.LookupBusinessRegistrationResponse], error) {
	// 格式與檢查碼先在這裡擋下，不打外部（輸入框打錯一碼是常態，不該消耗政府的公開 API）；額度已經在入口計過一次。
	taxID, ok := model.NormalizeTaxID(req.Msg.GetTaxId())
	if !ok {
		return nil, rpc.RequiredField("tax_id", "統一編號不正確：必須是 8 碼數字，而且檢查碼要符合規則")
	}
	reg, found, err := h.m.LookupBusinessRegistration(ctx, taxID)
	switch {
	case err != nil:
		return nil, err
	case !found:
		return connect.NewResponse(&pb.LookupBusinessRegistrationResponse{}), nil
	}
	out := &pb.BusinessRegistration{
		TaxId: reg.TaxID, Name: reg.Name, Address: reg.Address, OrganizationType: reg.OrganizationType,
	}
	if reg.TownCode != "" {
		out.PostalAddress = &commonpb.PostalAddress{TownCode: reg.TownCode, AddressDetail: reg.AddressDetail}
	}
	if !reg.SetupDate.IsZero() {
		out.SetupDate = &date.Date{Year: int32(reg.SetupDate.Year()), Month: int32(reg.SetupDate.Month()), Day: int32(reg.SetupDate.Day())} //nolint:gosec // time.Time 的日曆欄位在 Date 的範圍內
	}
	return connect.NewResponse(&pb.LookupBusinessRegistrationResponse{Registration: out}), nil
}
