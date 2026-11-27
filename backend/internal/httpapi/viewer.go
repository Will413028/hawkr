package httpapi

import (
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
)

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

	"/hawkr.review.v1alpha1.ReviewPublicService/ListProductReviews":    true,
	"/hawkr.review.v1alpha1.ReviewPublicService/ListStoreReviews":      true,
	"/hawkr.review.v1alpha1.ReviewPublicService/GetStoreReviewStats":   false,
	"/hawkr.review.v1alpha1.ReviewPublicService/GetProductReviewStats": false,

	"/hawkr.content.v1alpha1.ContentPublicService/ListLiveBanners": false,
	"/hawkr.content.v1alpha1.ContentPublicService/CheckAppVersion": false,

	"/hawkr.catalog.v1alpha1.CatalogPublicService/ListProducts":            false,
	"/hawkr.catalog.v1alpha1.CatalogPublicService/SearchProducts":          false,
	"/hawkr.catalog.v1alpha1.CatalogPublicService/SuggestProductNames":     false,
	"/hawkr.catalog.v1alpha1.CatalogPublicService/GetProduct":              false,
	"/hawkr.catalog.v1alpha1.CatalogPublicService/ListCategories":          false,
	"/hawkr.catalog.v1alpha1.CatalogPublicService/SuggestHashtags":         false,
	"/hawkr.catalog.v1alpha1.CatalogPublicService/SuggestTrendingHashtags": false,

	"/hawkr.seller.v1alpha1.SellerPublicService/GetStore":            false,
	"/hawkr.seller.v1alpha1.SellerPublicService/ListStores":          false,
	"/hawkr.seller.v1alpha1.SellerPublicService/BatchGetStores":      false,
	"/hawkr.seller.v1alpha1.SellerPublicService/GetSellerAgreements": false,
	"/hawkr.seller.v1alpha1.SellerPublicService/RecordProductView":   false,
}

func isViewerDependent(procedure string) bool { return publicViewerDependent[procedure] }

func checkPublicViewerDependence(services []service) error {
	descriptors, err := serviceDescriptors(services, "公開", func(a rpc.Audience) bool { return a == rpc.AudiencePublic })
	if err != nil {
		return err
	}
	return checkViewerDependenceTable("httpapi/viewer", rpc.WithoutUnmountedDeferred(publicViewerDependent, descriptors), descriptors)
}

func checkViewerDependenceTable(scope string, table map[string]bool, descriptors []protoreflect.ServiceDescriptor) error {
	return rpc.CheckDeclarations(scope, table, descriptors, func(bool) string { return "" })
}
