package httpapi

import (
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
)

// publicViewerDependent 宣告每個 *PublicService 的 procedure 的回應會不會依觀看者而不同（讀呼叫者的 session：
// viewer_can_*、is_mine、封鎖過濾），key 是 procedure（/<service 全名>/<方法>）。依觀看者而定、而且請求帶憑證的 GET，
// 回應是 private, no-store（cachePolicy）；不依觀看者的照常 public, no-cache。新增的 Public procedure 必須在這裡宣告，
// 缺宣告、多宣告都不能啟動（checkPublicViewerDependence）。
var publicViewerDependent = map[string]bool{
	"/hawkr.identity.v1alpha1.IdentityPublicService/IssueAppLoginNonce": false,
	"/hawkr.identity.v1alpha1.IdentityPublicService/ExchangeIdToken":    false,

	"/hawkr.discovery.v1alpha1.DiscoveryPublicService/SuggestRecommendedSellers": false,

	"/hawkr.social.v1alpha1.SocialPublicService/GetPost":              true,
	"/hawkr.social.v1alpha1.SocialPublicService/ListPosts":            true,
	"/hawkr.social.v1alpha1.SocialPublicService/ListComments":         true,
	"/hawkr.social.v1alpha1.SocialPublicService/ListForumEntries":     true,
	"/hawkr.social.v1alpha1.SocialPublicService/GetForumThread":       true,
	"/hawkr.social.v1alpha1.SocialPublicService/SuggestTrendingPosts": true,
	"/hawkr.social.v1alpha1.SocialPublicService/SuggestTags":          false,
	"/hawkr.social.v1alpha1.SocialPublicService/SuggestTrendingTags":  false,
	"/hawkr.social.v1alpha1.SocialPublicService/RecordPostView":       false,

	"/hawkr.seller.v1alpha1.SellerReportPublicService/SubmitFoodSafetyReport": false,

	"/hawkr.content.v1alpha1.ContentPublicService/ListLiveBanners": false,
	"/hawkr.content.v1alpha1.ContentPublicService/CheckAppVersion": false,

	"/hawkr.seller.v1alpha1.SellerPublicService/GetStore":            false,
	"/hawkr.seller.v1alpha1.SellerPublicService/ListStores":          false,
	"/hawkr.seller.v1alpha1.SellerPublicService/BatchGetStores":      false,
	"/hawkr.seller.v1alpha1.SellerPublicService/GetSellerAgreements": false,
	"/hawkr.seller.v1alpha1.SellerPublicService/RecordProductView":   false,
}

func isViewerDependent(procedure string) bool { return publicViewerDependent[procedure] }

// checkPublicViewerDependence 對照 publicViewerDependent 與實際掛載的 Public service，缺宣告或多宣告都不能啟動。
func checkPublicViewerDependence(services []service) error {
	descriptors, err := serviceDescriptors(services, "公開", func(a rpc.Audience) bool { return a == rpc.AudiencePublic })
	if err != nil {
		return err
	}
	return checkViewerDependenceTable("httpapi/viewer", publicViewerDependent, descriptors)
}

func checkViewerDependenceTable(scope string, table map[string]bool, descriptors []protoreflect.ServiceDescriptor) error {
	return rpc.CheckDeclarations(scope, table, descriptors, func(bool) string { return "" })
}
