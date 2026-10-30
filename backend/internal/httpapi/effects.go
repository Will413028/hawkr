package httpapi

import (
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
)

// Effect 是顧客 procedure 的動作性質：停權的帳號能不能呼叫它（A7-D18）。零值不是合法的宣告。
type Effect int

const (
	// EffectRead 不改變任何狀態：停權的帳號可以呼叫。
	EffectRead Effect = iota + 1
	// EffectWrite 改變狀態：停權的帳號呼叫時回 PermissionDenied 與 ErrorReason ACCOUNT_SUSPENDED。
	EffectWrite
	// EffectSelfService 是帳號對自己的處置，停權時也必須能做（例如刪除帳號）：停權的帳號可以呼叫。
	EffectSelfService
)

func (e Effect) valid() bool { return e == EffectRead || e == EffectWrite || e == EffectSelfService }

// customerEffects 宣告每個顧客 service（*AppService、*PublicService、*InviteeService，不含 admin）的每個 procedure 的動作性質，
// 由 withSession 執行；key 是 procedure（/<service 全名>/<方法>）。與賣家的能力宣告（店家的狀態）是不同的關注點，不合併。
// 公開與受邀供應商的 service 對停權的帳號一律以訪客執行，宣告仍要誠實標示方法的性質。
// 新增 procedure 必須在這裡宣告，缺宣告、多宣告都不能啟動（checkCustomerEffects）。
var customerEffects = map[string]Effect{
	// BuyerAppService
	"/hawkr.buyer.v1alpha1.BuyerAppService/GetProfile":               EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/UpdateProfile":            EffectWrite,
	"/hawkr.buyer.v1alpha1.BuyerAppService/ListAddresses":            EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/GetAddress":               EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/CreateAddress":            EffectWrite,
	"/hawkr.buyer.v1alpha1.BuyerAppService/UpdateAddress":            EffectWrite,
	"/hawkr.buyer.v1alpha1.BuyerAppService/DeleteAddress":            EffectWrite,
	"/hawkr.buyer.v1alpha1.BuyerAppService/ListFavorites":            EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/CheckFavorite":            EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/BatchCheckFavorites":      EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/AddFavorite":              EffectWrite,
	"/hawkr.buyer.v1alpha1.BuyerAppService/RemoveFavorite":           EffectWrite,
	"/hawkr.buyer.v1alpha1.BuyerAppService/ListFollows":              EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/CheckFollow":              EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/BatchCheckFollows":        EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/FollowStore":              EffectWrite,
	"/hawkr.buyer.v1alpha1.BuyerAppService/UnfollowStore":            EffectWrite,
	"/hawkr.buyer.v1alpha1.BuyerAppService/ListMemberCards":          EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/ListSpendingRecords":      EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/ListVisits":               EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/RecordVisit":              EffectWrite,
	"/hawkr.buyer.v1alpha1.BuyerAppService/ClearVisits":              EffectWrite,
	"/hawkr.buyer.v1alpha1.BuyerAppService/ListRestockSubscriptions": EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/SubscribeRestock":         EffectWrite,
	"/hawkr.buyer.v1alpha1.BuyerAppService/UnsubscribeRestock":       EffectWrite,

	// IdentityAppService
	"/hawkr.identity.v1alpha1.IdentityAppService/GetSession":           EffectRead,
	"/hawkr.identity.v1alpha1.IdentityAppService/DeleteAccount":        EffectSelfService,
	"/hawkr.identity.v1alpha1.IdentityAppService/SignOut":              EffectSelfService,
	"/hawkr.identity.v1alpha1.IdentityAppService/LinkIdentity":         EffectSelfService,
	"/hawkr.identity.v1alpha1.IdentityAppService/ListSessions":         EffectRead,
	"/hawkr.identity.v1alpha1.IdentityAppService/ListLinkedIdentities": EffectRead,
	"/hawkr.identity.v1alpha1.IdentityAppService/RevokeSession":        EffectSelfService,

	// IdentityPublicService
	"/hawkr.identity.v1alpha1.IdentityPublicService/IssueAppLoginNonce": EffectWrite,
	"/hawkr.identity.v1alpha1.IdentityPublicService/ExchangeIdToken":    EffectWrite,

	// DiscoveryPublicService
	"/hawkr.discovery.v1alpha1.DiscoveryPublicService/SuggestRecommendedSellers": EffectRead,

	// SellerReportAppService
	"/hawkr.seller.v1alpha1.SellerReportAppService/CreateReport":  EffectWrite,
	"/hawkr.seller.v1alpha1.SellerReportAppService/ListMyReports": EffectRead,

	// SellerReportPublicService
	"/hawkr.seller.v1alpha1.SellerReportPublicService/SubmitFoodSafetyReport": EffectWrite,

	// VendorPortalPublicService
	"/hawkr.agent.v1alpha1.VendorPortalPublicService/PreviewInvite":  EffectRead,
	"/hawkr.agent.v1alpha1.VendorPortalPublicService/ExchangeInvite": EffectWrite,

	// VendorPortalInviteeService
	"/hawkr.agent.v1alpha1.VendorPortalInviteeService/GetSession":    EffectRead,
	"/hawkr.agent.v1alpha1.VendorPortalInviteeService/VerifySession": EffectWrite,

	// ContentPublicService
	"/hawkr.content.v1alpha1.ContentPublicService/ListLiveBanners": EffectRead,
	"/hawkr.content.v1alpha1.ContentPublicService/CheckAppVersion": EffectRead,

	// SellerAppService
	"/hawkr.seller.v1alpha1.SellerAppService/ApplyStore":                 EffectWrite,
	"/hawkr.seller.v1alpha1.SellerAppService/GetMyStoreOpening":          EffectRead,
	"/hawkr.seller.v1alpha1.SellerAppService/GetMyStore":                 EffectRead,
	"/hawkr.seller.v1alpha1.SellerAppService/UpdateMyStore":              EffectWrite,
	"/hawkr.seller.v1alpha1.SellerAppService/CompleteMyStoreOnboarding":  EffectWrite,
	"/hawkr.seller.v1alpha1.SellerAppService/ListMyFollowers":            EffectRead,
	"/hawkr.seller.v1alpha1.SellerAppService/GetMyDashboard":             EffectRead,
	"/hawkr.seller.v1alpha1.SellerAppService/CloseMyStore":               EffectWrite,
	"/hawkr.seller.v1alpha1.SellerAppService/SaveKYCDraft":               EffectWrite,
	"/hawkr.seller.v1alpha1.SellerAppService/GetMyKYC":                   EffectRead,
	"/hawkr.seller.v1alpha1.SellerAppService/SubmitKYC":                  EffectWrite,
	"/hawkr.seller.v1alpha1.SellerAppService/CancelKYC":                  EffectWrite,
	"/hawkr.seller.v1alpha1.SellerAppService/LookupBusinessRegistration": EffectRead,
	"/hawkr.seller.v1alpha1.SellerAppService/ListMyMeasures":             EffectRead,
	"/hawkr.seller.v1alpha1.SellerAppService/GetMyMeasure":               EffectRead,
	"/hawkr.seller.v1alpha1.SellerAppService/SubmitAppeal":               EffectWrite,
	"/hawkr.seller.v1alpha1.SellerAppService/RequestRemediation":         EffectWrite,

	// SellerPublicService
	"/hawkr.seller.v1alpha1.SellerPublicService/GetStore":            EffectRead,
	"/hawkr.seller.v1alpha1.SellerPublicService/ListStores":          EffectRead,
	"/hawkr.seller.v1alpha1.SellerPublicService/BatchGetStores":      EffectRead,
	"/hawkr.seller.v1alpha1.SellerPublicService/GetSellerAgreements": EffectRead,
	"/hawkr.seller.v1alpha1.SellerPublicService/RecordProductView":   EffectWrite,
}

// effectOf 回傳 procedure 宣告的動作性質；沒有宣告的當寫入（預設拒絕）。
func effectOf(procedure string) Effect {
	if e, ok := customerEffects[procedure]; ok {
		return e
	}
	return EffectWrite
}

// customerServiceDescriptors 是掛載的 service 中 audience 不是 admin 的 descriptor，查法同 adminServiceDescriptors。
func customerServiceDescriptors(services []service) ([]protoreflect.ServiceDescriptor, error) {
	return serviceDescriptors(services, "顧客", func(a rpc.Audience) bool { return a != rpc.AudienceAdmin })
}

// checkCustomerEffects 對照 customerEffects 與實際掛載的顧客 service：掛了沒有宣告的方法、宣告了沒有掛載的方法、宣告的值不合法，
// 都不能啟動。
func checkCustomerEffects(services []service) error {
	descriptors, err := customerServiceDescriptors(services)
	if err != nil {
		return err
	}
	return checkCustomerEffectTable("httpapi/effects", customerEffects, descriptors)
}

func checkCustomerEffectTable(scope string, table map[string]Effect, descriptors []protoreflect.ServiceDescriptor) error {
	return rpc.CheckDeclarations(scope, table, descriptors, func(e Effect) string {
		if !e.valid() {
			return fmt.Sprintf("的動作性質 %d 不合法", int(e))
		}
		return ""
	})
}
