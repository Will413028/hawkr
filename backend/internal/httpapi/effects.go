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

var customerEffects = map[string]Effect{
	"/hawkr.buyer.v1alpha1.BuyerAppService/GetProfile":                EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/UpdateProfile":             EffectWrite,
	"/hawkr.buyer.v1alpha1.BuyerAppService/ListAddresses":             EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/GetAddress":                EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/CreateAddress":             EffectWrite,
	"/hawkr.buyer.v1alpha1.BuyerAppService/UpdateAddress":             EffectWrite,
	"/hawkr.buyer.v1alpha1.BuyerAppService/DeleteAddress":             EffectWrite,
	"/hawkr.buyer.v1alpha1.BuyerAppService/ListFavorites":             EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/CheckFavorite":             EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/BatchCheckFavorites":       EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/AddFavorite":               EffectWrite,
	"/hawkr.buyer.v1alpha1.BuyerAppService/RemoveFavorite":            EffectWrite,
	"/hawkr.buyer.v1alpha1.BuyerAppService/ListFollowedStoreProducts": EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/ListFollows":               EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/CheckFollow":               EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/BatchCheckFollows":         EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/FollowStore":               EffectWrite,
	"/hawkr.buyer.v1alpha1.BuyerAppService/UnfollowStore":             EffectWrite,
	"/hawkr.buyer.v1alpha1.BuyerAppService/ListMemberCards":           EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/ListSpendingRecords":       EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/ListVisits":                EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/RecordVisit":               EffectWrite,
	"/hawkr.buyer.v1alpha1.BuyerAppService/ClearVisits":               EffectWrite,
	"/hawkr.buyer.v1alpha1.BuyerAppService/ListRestockSubscriptions":  EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/SubscribeRestock":          EffectWrite,
	"/hawkr.buyer.v1alpha1.BuyerAppService/UnsubscribeRestock":        EffectWrite,
	"/hawkr.buyer.v1alpha1.BuyerAppService/BlockAuthor":               EffectSelfService,
	"/hawkr.buyer.v1alpha1.BuyerAppService/BlockStore":                EffectSelfService,
	"/hawkr.buyer.v1alpha1.BuyerAppService/UnblockBuyer":              EffectSelfService,
	"/hawkr.buyer.v1alpha1.BuyerAppService/UnblockStore":              EffectSelfService,
	"/hawkr.buyer.v1alpha1.BuyerAppService/ListBlockedBuyers":         EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/ListBlockedStores":         EffectRead,

	"/hawkr.payment.v1alpha1.PaymentAppService/CreatePayment": EffectWrite,
	"/hawkr.payment.v1alpha1.PaymentAppService/GetPayment":    EffectRead,

	"/hawkr.assistant.v1alpha1.AssistantAppService/SendAssistantMessage":                EffectWrite,
	"/hawkr.assistant.v1alpha1.AssistantAppService/WatchAssistantTurn":                  EffectRead,
	"/hawkr.assistant.v1alpha1.AssistantAppService/GetLatestAssistantSession":           EffectRead,
	"/hawkr.imageai.v1alpha1.ImageAiAppService/TransformImage":                          EffectWrite,
	"/hawkr.imageai.v1alpha1.ImageAiAppService/ListAiStyles":                            EffectRead,
	"/hawkr.procurement.v1alpha1.BuyerProcurementAppService/SubmitTurn":                 EffectWrite,
	"/hawkr.procurement.v1alpha1.BuyerProcurementAppService/WatchRun":                   EffectRead,
	"/hawkr.procurement.v1alpha1.BuyerProcurementAppService/CancelRun":                  EffectWrite,
	"/hawkr.procurement.v1alpha1.BuyerProcurementAppService/ResumeRun":                  EffectWrite,
	"/hawkr.procurement.v1alpha1.BuyerProcurementAppService/StartCandidateSearch":       EffectWrite,
	"/hawkr.procurement.v1alpha1.BuyerProcurementAppService/GetCandidateSearch":         EffectRead,
	"/hawkr.procurement.v1alpha1.BuyerProcurementAppService/ToggleSelection":            EffectWrite,
	"/hawkr.procurement.v1alpha1.BuyerProcurementAppService/SubmitAnswer":               EffectWrite,
	"/hawkr.procurement.v1alpha1.BuyerProcurementAppService/Rewind":                     EffectWrite,
	"/hawkr.procurement.v1alpha1.BuyerProcurementAppService/PreviewRfqPrice":            EffectRead,
	"/hawkr.procurement.v1alpha1.BuyerProcurementAppService/PreviewRfqPublication":      EffectWrite,
	"/hawkr.procurement.v1alpha1.BuyerProcurementAppService/ExecuteRfqPublication":      EffectWrite,
	"/hawkr.procurement.v1alpha1.BuyerProcurementAppService/GetSnapshot":                EffectRead,
	"/hawkr.procurement.v1alpha1.BuyerProcurementAppService/GetRuntimeSnapshot":         EffectRead,
	"/hawkr.procurement.v1alpha1.BuyerProcurementAppService/ListConversations":          EffectRead,
	"/hawkr.procurement.v1alpha1.BuyerProcurementAppService/GetConversationMessages":    EffectRead,
	"/hawkr.procurement.v1alpha1.BuyerProcurementAppService/RenameConversation":         EffectWrite,
	"/hawkr.procurement.v1alpha1.BuyerProcurementAppService/DeleteConversation":         EffectWrite,
	"/hawkr.procurement.v1alpha1.BuyerProcurementAppService/ListSellerResponses":        EffectRead,
	"/hawkr.procurement.v1alpha1.BuyerProcurementAppService/PutShortlistEntry":          EffectWrite,
	"/hawkr.procurement.v1alpha1.BuyerProcurementAppService/DeleteShortlistEntry":       EffectWrite,
	"/hawkr.procurement.v1alpha1.BuyerProcurementAppService/AcceptSellerQuote":          EffectWrite,
	"/hawkr.procurement.v1alpha1.SellerProcurementAppService/ListSellerPublications":    EffectRead,
	"/hawkr.procurement.v1alpha1.SellerProcurementAppService/GetSellerPublication":      EffectRead,
	"/hawkr.procurement.v1alpha1.SellerProcurementAppService/UpsertSellerResponseDraft": EffectWrite,
	"/hawkr.procurement.v1alpha1.SellerProcurementAppService/SubmitSellerResponse":      EffectWrite,
	"/hawkr.procurement.v1alpha1.SellerProcurementAppService/WithdrawSellerResponse":    EffectWrite,
	"/hawkr.procurement.v1alpha1.SellerProcurementAppService/DeclineRfqPublication":     EffectWrite,

	"/hawkr.identity.v1alpha1.IdentityAppService/GetSession":           EffectRead,
	"/hawkr.identity.v1alpha1.IdentityAppService/DeleteAccount":        EffectSelfService,
	"/hawkr.identity.v1alpha1.IdentityAppService/SignOut":              EffectSelfService,
	"/hawkr.identity.v1alpha1.IdentityAppService/LinkIdentity":         EffectSelfService,
	"/hawkr.identity.v1alpha1.IdentityAppService/ListSessions":         EffectRead,
	"/hawkr.identity.v1alpha1.IdentityAppService/ListLinkedIdentities": EffectRead,
	"/hawkr.identity.v1alpha1.IdentityAppService/RevokeSession":        EffectSelfService,
	"/hawkr.identity.v1alpha1.IdentityAppService/GetContactEmail":      EffectRead,
	"/hawkr.identity.v1alpha1.IdentityAppService/SetContactEmail":      EffectWrite,

	"/hawkr.identity.v1alpha1.IdentityPublicService/IssueAppLoginNonce": EffectWrite,
	"/hawkr.identity.v1alpha1.IdentityPublicService/ExchangeIdToken":    EffectWrite,

	"/hawkr.discovery.v1alpha1.DiscoveryPublicService/SuggestRecommendedSellers": EffectRead,

	"/hawkr.social.v1alpha1.SocialPublicService/GetPost":              EffectRead,
	"/hawkr.social.v1alpha1.SocialPublicService/ListPosts":            EffectRead,
	"/hawkr.social.v1alpha1.SocialPublicService/ListComments":         EffectRead,
	"/hawkr.social.v1alpha1.SocialPublicService/SuggestTrendingPosts": EffectRead,
	"/hawkr.social.v1alpha1.SocialPublicService/SuggestTags":          EffectRead,
	"/hawkr.social.v1alpha1.SocialPublicService/SuggestTrendingTags":  EffectRead,
	"/hawkr.social.v1alpha1.SocialPublicService/ListForumEntries":     EffectRead,
	"/hawkr.social.v1alpha1.SocialPublicService/GetForumThread":       EffectRead,
	"/hawkr.social.v1alpha1.SocialPublicService/RecordPostView":       EffectWrite,

	"/hawkr.moderation.v1alpha1.ModerationAppService/SubmitReport":       EffectWrite,
	"/hawkr.moderation.v1alpha1.ModerationAppService/ListMyReports":      EffectRead,
	"/hawkr.moderation.v1alpha1.ModerationAppService/ListMyStoreReports": EffectRead,

	"/hawkr.media.v1alpha1.MediaAppService/BeginUpload":             EffectWrite,
	"/hawkr.media.v1alpha1.MediaAppService/CompleteUpload":          EffectWrite,
	"/hawkr.media.v1alpha1.MediaAppService/GetUpload":               EffectRead,
	"/hawkr.media.v1alpha1.MediaAppService/BeginSecureUpload":       EffectWrite,
	"/hawkr.media.v1alpha1.MediaDelegatedService/BeginSecureUpload": EffectWrite,
	"/hawkr.media.v1alpha1.MediaDelegatedService/BeginUpload":       EffectWrite,
	"/hawkr.media.v1alpha1.MediaDelegatedService/CompleteUpload":    EffectWrite,
	"/hawkr.media.v1alpha1.MediaDelegatedService/GetUpload":         EffectRead,

	"/hawkr.social.v1alpha1.SocialAppService/SharePost":             EffectWrite,
	"/hawkr.social.v1alpha1.SocialAppService/LikePost":              EffectWrite,
	"/hawkr.social.v1alpha1.SocialAppService/UnlikePost":            EffectWrite,
	"/hawkr.social.v1alpha1.SocialAppService/LikeComment":           EffectWrite,
	"/hawkr.social.v1alpha1.SocialAppService/UnlikeComment":         EffectWrite,
	"/hawkr.social.v1alpha1.SocialAppService/CreateComment":         EffectWrite,
	"/hawkr.social.v1alpha1.SocialAppService/UpdateComment":         EffectWrite,
	"/hawkr.social.v1alpha1.SocialAppService/DeleteComment":         EffectWrite,
	"/hawkr.social.v1alpha1.SocialAppService/CreateForumEntry":      EffectWrite,
	"/hawkr.social.v1alpha1.SocialAppService/UpdateForumEntry":      EffectWrite,
	"/hawkr.social.v1alpha1.SocialAppService/DeleteForumEntry":      EffectWrite,
	"/hawkr.social.v1alpha1.SocialAppService/CreatePost":            EffectWrite,
	"/hawkr.social.v1alpha1.SocialAppService/GetMyPost":             EffectRead,
	"/hawkr.social.v1alpha1.SocialAppService/ListMyPosts":           EffectRead,
	"/hawkr.social.v1alpha1.SocialAppService/UpdatePost":            EffectWrite,
	"/hawkr.social.v1alpha1.SocialAppService/PublishPost":           EffectWrite,
	"/hawkr.social.v1alpha1.SocialAppService/UnpublishPost":         EffectWrite,
	"/hawkr.social.v1alpha1.SocialAppService/DeletePost":            EffectWrite,
	"/hawkr.social.v1alpha1.SocialAppService/SchedulePost":          EffectWrite,
	"/hawkr.social.v1alpha1.SocialAppService/ListPendingSchedules":  EffectRead,
	"/hawkr.social.v1alpha1.SocialAppService/CancelPostSchedule":    EffectWrite,
	"/hawkr.social.v1alpha1.SocialAppService/ListStoreComments":     EffectRead,
	"/hawkr.social.v1alpha1.SocialAppService/GetStoreSocialSummary": EffectRead,
	"/hawkr.social.v1alpha1.SocialAppService/ListMySanctions":       EffectRead,
	"/hawkr.social.v1alpha1.SocialAppService/ListMyHiddenContent":   EffectRead,

	"/hawkr.review.v1alpha1.ReviewPublicService/ListProductReviews":    EffectRead,
	"/hawkr.review.v1alpha1.ReviewPublicService/ListStoreReviews":      EffectRead,
	"/hawkr.review.v1alpha1.ReviewPublicService/GetStoreReviewStats":   EffectRead,
	"/hawkr.review.v1alpha1.ReviewPublicService/GetProductReviewStats": EffectRead,

	"/hawkr.review.v1alpha1.ReviewAppService/GetReview":           EffectRead,
	"/hawkr.review.v1alpha1.ReviewAppService/ReplyReview":         EffectWrite,
	"/hawkr.review.v1alpha1.ReviewAppService/CreateReview":        EffectWrite,
	"/hawkr.review.v1alpha1.ReviewAppService/ListMyReviews":       EffectRead,
	"/hawkr.review.v1alpha1.ReviewAppService/ListOwnStoreReviews": EffectRead,

	"/hawkr.coupon.v1alpha1.CouponAppService/ListMyVouchers":         EffectRead,
	"/hawkr.coupon.v1alpha1.CouponAppService/ListClaimableCampaigns": EffectRead,
	"/hawkr.coupon.v1alpha1.CouponAppService/ClaimCoupon":            EffectWrite,
	"/hawkr.coupon.v1alpha1.CouponAppService/CreateCampaign":         EffectWrite,
	"/hawkr.coupon.v1alpha1.CouponAppService/ListCampaigns":          EffectRead,
	"/hawkr.coupon.v1alpha1.CouponAppService/GetCampaign":            EffectRead,
	"/hawkr.coupon.v1alpha1.CouponAppService/EndCampaign":            EffectWrite,
	"/hawkr.crm.v1alpha1.CrmAppService/ListCustomers":                EffectRead,
	"/hawkr.crm.v1alpha1.CrmAppService/GetCustomer":                  EffectRead,
	"/hawkr.crm.v1alpha1.CrmAppService/UpdateCustomer":               EffectWrite,
	"/hawkr.crm.v1alpha1.CrmAppService/CreateTag":                    EffectWrite,
	"/hawkr.crm.v1alpha1.CrmAppService/ListTags":                     EffectRead,
	"/hawkr.crm.v1alpha1.CrmAppService/UpdateTag":                    EffectWrite,
	"/hawkr.crm.v1alpha1.CrmAppService/DeleteTag":                    EffectWrite,
	"/hawkr.crm.v1alpha1.CrmAppService/AddCustomerTags":              EffectWrite,
	"/hawkr.crm.v1alpha1.CrmAppService/RemoveCustomerTag":            EffectWrite,

	"/hawkr.chat.v1alpha1.ChatAppService/ListConversations":           EffectRead,
	"/hawkr.chat.v1alpha1.ChatAppService/ListStoreConversations":      EffectRead,
	"/hawkr.chat.v1alpha1.ChatAppService/GetStoreConversationByBuyer": EffectRead,
	"/hawkr.chat.v1alpha1.ChatAppService/ListMessages":                EffectRead,
	"/hawkr.chat.v1alpha1.ChatAppService/SendMessage":                 EffectWrite,
	"/hawkr.chat.v1alpha1.ChatAppService/SendMessageToStore":          EffectWrite,
	"/hawkr.chat.v1alpha1.ChatAppService/SendMessageToBuyer":          EffectWrite,
	"/hawkr.chat.v1alpha1.ChatAppService/MarkConversationRead":        EffectWrite,
	"/hawkr.chat.v1alpha1.ChatAppService/MarkAllRead":                 EffectWrite,
	"/hawkr.chat.v1alpha1.ChatAppService/MarkAllStoreRead":            EffectWrite,
	"/hawkr.chat.v1alpha1.ChatAppService/GetUnreadCount":              EffectRead,
	"/hawkr.chat.v1alpha1.ChatAppService/GetStoreUnreadCount":         EffectRead,
	"/hawkr.chat.v1alpha1.ChatAppService/GetPresence":                 EffectRead,
	"/hawkr.chat.v1alpha1.ChatAppService/GetStorePresence":            EffectRead,
	"/hawkr.chat.v1alpha1.ChatAppService/StreamEvents":                EffectRead,
	"/hawkr.chat.v1alpha1.ChatAppService/StreamStoreEvents":           EffectRead,

	"/hawkr.notification.v1alpha1.NotificationAppService/ListInboxItems":               EffectRead,
	"/hawkr.notification.v1alpha1.NotificationAppService/GetUnreadCount":               EffectRead,
	"/hawkr.notification.v1alpha1.NotificationAppService/MarkInboxItemRead":            EffectSelfService,
	"/hawkr.notification.v1alpha1.NotificationAppService/MarkAllInboxItemsRead":        EffectSelfService,
	"/hawkr.notification.v1alpha1.NotificationAppService/BatchDeleteInboxItems":        EffectWrite,
	"/hawkr.notification.v1alpha1.NotificationAppService/GetNotificationPreferences":   EffectRead,
	"/hawkr.notification.v1alpha1.NotificationAppService/UpdateNotificationPreference": EffectWrite,
	"/hawkr.notification.v1alpha1.NotificationAppService/RegisterDeviceToken":          EffectWrite,
	"/hawkr.notification.v1alpha1.NotificationAppService/DeleteDeviceToken":            EffectSelfService,
	"/hawkr.notification.v1alpha1.NotificationAppService/WatchNotifications":           EffectRead,

	"/hawkr.catalog.v1alpha1.CatalogPublicService/ListProducts":            EffectRead,
	"/hawkr.catalog.v1alpha1.CatalogPublicService/SearchProducts":          EffectRead,
	"/hawkr.catalog.v1alpha1.CatalogPublicService/SuggestProductNames":     EffectRead,
	"/hawkr.catalog.v1alpha1.CatalogPublicService/GetProduct":              EffectRead,
	"/hawkr.catalog.v1alpha1.CatalogPublicService/ListCategories":          EffectRead,
	"/hawkr.catalog.v1alpha1.CatalogPublicService/SuggestHashtags":         EffectRead,
	"/hawkr.catalog.v1alpha1.CatalogPublicService/SuggestTrendingHashtags": EffectRead,

	"/hawkr.catalog.v1alpha1.SellerCatalogAppService/ListMyProducts":   EffectRead,
	"/hawkr.catalog.v1alpha1.SellerCatalogAppService/GetMyProduct":     EffectRead,
	"/hawkr.catalog.v1alpha1.SellerCatalogAppService/CountMyProducts":  EffectRead,
	"/hawkr.catalog.v1alpha1.SellerCatalogAppService/CreateProduct":    EffectWrite,
	"/hawkr.catalog.v1alpha1.SellerCatalogAppService/UpdateProduct":    EffectWrite,
	"/hawkr.catalog.v1alpha1.SellerCatalogAppService/SetProductStatus": EffectWrite,
	"/hawkr.catalog.v1alpha1.SellerCatalogAppService/DeleteProduct":    EffectWrite,
	"/hawkr.catalog.v1alpha1.SellerCatalogAppService/AdjustStock":      EffectWrite,
	"/hawkr.catalog.v1alpha1.SellerCatalogAppService/SetStock":         EffectWrite,

	"/hawkr.seller.v1alpha1.SellerReportAppService/CreateReport":        EffectWrite,
	"/hawkr.seller.v1alpha1.SellerReportAppService/GetMyReport":         EffectRead,
	"/hawkr.seller.v1alpha1.SellerReportAppService/AddReportSupplement": EffectWrite,

	"/hawkr.social.v1alpha1.SocialDelegatedService/GetStoreSocialSummary": EffectRead,
	"/hawkr.social.v1alpha1.SocialDelegatedService/ListStorePosts":        EffectRead,
	"/hawkr.social.v1alpha1.SocialDelegatedService/GetStorePost":          EffectRead,
	"/hawkr.social.v1alpha1.SocialDelegatedService/CreateStorePost":       EffectWrite,
	"/hawkr.social.v1alpha1.SocialDelegatedService/UpdateStorePost":       EffectWrite,
	"/hawkr.social.v1alpha1.SocialDelegatedService/PublishStorePost":      EffectWrite,
	"/hawkr.social.v1alpha1.SocialDelegatedService/UnpublishStorePost":    EffectWrite,
	"/hawkr.social.v1alpha1.SocialDelegatedService/ScheduleStorePost":     EffectWrite,

	"/hawkr.content.v1alpha1.ContentPublicService/ListLiveBanners": EffectRead,
	"/hawkr.content.v1alpha1.ContentPublicService/CheckAppVersion": EffectRead,

	"/hawkr.seller.v1alpha1.SellerAppService/ApplyStore":                     EffectWrite,
	"/hawkr.seller.v1alpha1.SellerAppService/GetMyStoreOpening":              EffectRead,
	"/hawkr.seller.v1alpha1.SellerAppService/GetMyStore":                     EffectRead,
	"/hawkr.seller.v1alpha1.SellerAppService/GetMyOnboarding":                EffectRead,
	"/hawkr.seller.v1alpha1.SellerAppService/RefreshMyPaymentAccount":        EffectWrite,
	"/hawkr.seller.v1alpha1.SellerAppService/UpdateMyStore":                  EffectWrite,
	"/hawkr.seller.v1alpha1.SellerAppService/CompleteMyStoreOnboarding":      EffectWrite,
	"/hawkr.seller.v1alpha1.SellerAppService/ListMyFollowers":                EffectRead,
	"/hawkr.seller.v1alpha1.SellerAppService/GetMyDashboard":                 EffectRead,
	"/hawkr.seller.v1alpha1.SellerAppService/CloseMyStore":                   EffectWrite,
	"/hawkr.seller.v1alpha1.SellerAppService/SaveKYCDraft":                   EffectWrite,
	"/hawkr.seller.v1alpha1.SellerAppService/GetMyKYC":                       EffectRead,
	"/hawkr.seller.v1alpha1.SellerAppService/SubmitKYC":                      EffectWrite,
	"/hawkr.seller.v1alpha1.SellerAppService/CancelKYC":                      EffectWrite,
	"/hawkr.seller.v1alpha1.SellerAppService/GetMyPaymentAccount":            EffectRead,
	"/hawkr.seller.v1alpha1.SellerAppService/ListMyEarnings":                 EffectRead,
	"/hawkr.seller.v1alpha1.SellerAppService/GetMyOrderEarnings":             EffectRead,
	"/hawkr.seller.v1alpha1.SellerAppService/CreatePaymentOnboardingSession": EffectWrite,
	"/hawkr.seller.v1alpha1.SellerAppService/LookupBusinessRegistration":     EffectRead,
	"/hawkr.seller.v1alpha1.SellerAppService/ListMyMeasures":                 EffectRead,
	"/hawkr.seller.v1alpha1.SellerAppService/GetMyMeasure":                   EffectRead,
	"/hawkr.seller.v1alpha1.SellerAppService/SubmitAppeal":                   EffectWrite,
	"/hawkr.seller.v1alpha1.SellerAppService/RequestRemediation":             EffectWrite,
	"/hawkr.seller.v1alpha1.SellerAppService/ListMyInquiries":                EffectRead,
	"/hawkr.seller.v1alpha1.SellerAppService/GetMyInquiry":                   EffectRead,
	"/hawkr.seller.v1alpha1.SellerAppService/ReplyToInquiry":                 EffectWrite,
	"/hawkr.seller.v1alpha1.SellerAppService/BlockBuyer":                     EffectSelfService,
	"/hawkr.seller.v1alpha1.SellerAppService/UnblockBuyer":                   EffectSelfService,
	"/hawkr.seller.v1alpha1.SellerAppService/ListBlockedBuyers":              EffectRead,

	"/hawkr.seller.v1alpha1.SellerPublicService/GetStore":            EffectRead,
	"/hawkr.seller.v1alpha1.SellerPublicService/ListStores":          EffectRead,
	"/hawkr.seller.v1alpha1.SellerPublicService/BatchGetStores":      EffectRead,
	"/hawkr.seller.v1alpha1.SellerPublicService/GetSellerAgreements": EffectRead,
	"/hawkr.seller.v1alpha1.SellerPublicService/RecordProductView":   EffectWrite,
}

func effectOf(procedure string) Effect {
	if e, ok := customerEffects[procedure]; ok {
		return e
	}
	return EffectWrite
}

func customerServiceDescriptors(services []service) ([]protoreflect.ServiceDescriptor, error) {
	return serviceDescriptors(services, "顧客", func(a rpc.Audience) bool { return a != rpc.AudienceAdmin })
}

func checkCustomerEffects(services []service) error {
	descriptors, err := customerServiceDescriptors(services)
	if err != nil {
		return err
	}
	return checkCustomerEffectTable("httpapi/effects", rpc.WithoutUnmountedDeferred(customerEffects, descriptors), descriptors)
}

func checkCustomerEffectTable(scope string, table map[string]Effect, descriptors []protoreflect.ServiceDescriptor) error {
	return rpc.CheckDeclarations(scope, table, descriptors, func(e Effect) string {
		if !e.valid() {
			return fmt.Sprintf("的動作性質 %d 不合法", int(e))
		}
		return ""
	})
}
