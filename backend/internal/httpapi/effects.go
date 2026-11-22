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

// customerEffects 宣告每個顧客 service（*AppService、*PublicService，不含 admin）的每個 procedure 的動作性質，
// 由 withSession 執行；key 是 procedure（/<service 全名>/<方法>）。與賣家的能力宣告（店家的狀態）是不同的關注點，不合併。
// 公開的 service 對停權的帳號一律以訪客執行，宣告仍要誠實標示方法的性質。
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
	// 個人封鎖是保護自己的動作（seller 模組的店家封鎖同理：停權時仍要能封鎖與解除），停權的帳號也能做；名單是讀取。
	"/hawkr.buyer.v1alpha1.BuyerAppService/BlockAuthor":       EffectSelfService,
	"/hawkr.buyer.v1alpha1.BuyerAppService/BlockStore":        EffectSelfService,
	"/hawkr.buyer.v1alpha1.BuyerAppService/UnblockBuyer":      EffectSelfService,
	"/hawkr.buyer.v1alpha1.BuyerAppService/UnblockStore":      EffectSelfService,
	"/hawkr.buyer.v1alpha1.BuyerAppService/ListBlockedBuyers": EffectRead,
	"/hawkr.buyer.v1alpha1.BuyerAppService/ListBlockedStores": EffectRead,

	// IdentityAppService
	"/hawkr.identity.v1alpha1.IdentityAppService/GetSession":           EffectRead,
	"/hawkr.identity.v1alpha1.IdentityAppService/DeleteAccount":        EffectSelfService,
	"/hawkr.identity.v1alpha1.IdentityAppService/SignOut":              EffectSelfService,
	"/hawkr.identity.v1alpha1.IdentityAppService/LinkIdentity":         EffectSelfService,
	"/hawkr.identity.v1alpha1.IdentityAppService/ListSessions":         EffectRead,
	"/hawkr.identity.v1alpha1.IdentityAppService/ListLinkedIdentities": EffectRead,
	"/hawkr.identity.v1alpha1.IdentityAppService/RevokeSession":        EffectSelfService,
	"/hawkr.identity.v1alpha1.IdentityAppService/GetContactEmail":      EffectRead,
	"/hawkr.identity.v1alpha1.IdentityAppService/SetContactEmail":      EffectWrite,

	// IdentityPublicService
	"/hawkr.identity.v1alpha1.IdentityPublicService/IssueAppLoginNonce": EffectWrite,
	"/hawkr.identity.v1alpha1.IdentityPublicService/ExchangeIdToken":    EffectWrite,

	// DiscoveryPublicService
	"/hawkr.discovery.v1alpha1.DiscoveryPublicService/SuggestRecommendedSellers": EffectRead,

	// SocialPublicService
	"/hawkr.social.v1alpha1.SocialPublicService/GetPost":              EffectRead,
	"/hawkr.social.v1alpha1.SocialPublicService/ListPosts":            EffectRead,
	"/hawkr.social.v1alpha1.SocialPublicService/ListComments":         EffectRead,
	"/hawkr.social.v1alpha1.SocialPublicService/SuggestTrendingPosts": EffectRead,
	"/hawkr.social.v1alpha1.SocialPublicService/SuggestTags":          EffectRead,
	"/hawkr.social.v1alpha1.SocialPublicService/SuggestTrendingTags":  EffectRead,
	"/hawkr.social.v1alpha1.SocialPublicService/ListForumEntries":     EffectRead,
	"/hawkr.social.v1alpha1.SocialPublicService/GetForumThread":       EffectRead,
	"/hawkr.social.v1alpha1.SocialPublicService/RecordPostView":       EffectWrite,

	// ModerationAppService：檢舉是寫入（停權的帳號不能送出）；讀自己的檢舉停權的帳號也可以。
	"/hawkr.moderation.v1alpha1.ModerationAppService/SubmitReport":       EffectWrite,
	"/hawkr.moderation.v1alpha1.ModerationAppService/ListMyReports":      EffectRead,
	"/hawkr.moderation.v1alpha1.ModerationAppService/ListMyStoreReports": EffectRead,

	// SocialAppService
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
	// 作者讀自己的處分與被隱藏內容的理由（A7-D9）：只讀，停權的帳號也要讀得到。
	"/hawkr.social.v1alpha1.SocialAppService/ListMySanctions":     EffectRead,
	"/hawkr.social.v1alpha1.SocialAppService/ListMyHiddenContent": EffectRead,

	// ReviewPublicService：公開的 service 對停權的帳號一律以訪客執行，宣告仍標示方法的性質。
	"/hawkr.review.v1alpha1.ReviewPublicService/ListProductReviews":  EffectRead,
	"/hawkr.review.v1alpha1.ReviewPublicService/ListStoreReviews":    EffectRead,
	"/hawkr.review.v1alpha1.ReviewPublicService/GetStoreReviewStats": EffectRead,

	// ReviewAppService
	"/hawkr.review.v1alpha1.ReviewAppService/GetReview":           EffectRead,
	"/hawkr.review.v1alpha1.ReviewAppService/ReplyReview":         EffectWrite,
	"/hawkr.review.v1alpha1.ReviewAppService/CreateReview":        EffectWrite,
	"/hawkr.review.v1alpha1.ReviewAppService/ListMyReviews":       EffectRead,
	"/hawkr.review.v1alpha1.ReviewAppService/ListOwnStoreReviews": EffectRead,

	// CrmAppService
	"/hawkr.crm.v1alpha1.CrmAppService/ListCustomers":     EffectRead,
	"/hawkr.crm.v1alpha1.CrmAppService/GetCustomer":       EffectRead,
	"/hawkr.crm.v1alpha1.CrmAppService/UpdateCustomer":    EffectWrite,
	"/hawkr.crm.v1alpha1.CrmAppService/CreateTag":         EffectWrite,
	"/hawkr.crm.v1alpha1.CrmAppService/ListTags":          EffectRead,
	"/hawkr.crm.v1alpha1.CrmAppService/UpdateTag":         EffectWrite,
	"/hawkr.crm.v1alpha1.CrmAppService/DeleteTag":         EffectWrite,
	"/hawkr.crm.v1alpha1.CrmAppService/AddCustomerTags":   EffectWrite,
	"/hawkr.crm.v1alpha1.CrmAppService/RemoveCustomerTag": EffectWrite,

	// ChatAppService
	"/hawkr.chat.v1alpha1.ChatAppService/ListConversations":      EffectRead,
	"/hawkr.chat.v1alpha1.ChatAppService/ListStoreConversations": EffectRead,
	"/hawkr.chat.v1alpha1.ChatAppService/ListMessages":           EffectRead,
	"/hawkr.chat.v1alpha1.ChatAppService/SendMessage":            EffectWrite,
	"/hawkr.chat.v1alpha1.ChatAppService/SendMessageToStore":     EffectWrite,
	"/hawkr.chat.v1alpha1.ChatAppService/SendMessageToBuyer":     EffectWrite,
	"/hawkr.chat.v1alpha1.ChatAppService/MarkConversationRead":   EffectWrite,
	"/hawkr.chat.v1alpha1.ChatAppService/MarkAllRead":            EffectWrite,
	"/hawkr.chat.v1alpha1.ChatAppService/MarkAllStoreRead":       EffectWrite,
	"/hawkr.chat.v1alpha1.ChatAppService/GetUnreadCount":         EffectRead,
	"/hawkr.chat.v1alpha1.ChatAppService/GetStoreUnreadCount":    EffectRead,
	"/hawkr.chat.v1alpha1.ChatAppService/GetPresence":            EffectRead,
	"/hawkr.chat.v1alpha1.ChatAppService/GetStorePresence":       EffectRead,
	// 串流只讀：停權的帳號可讀，停權不結束串流（A6-D16 W4）。
	"/hawkr.chat.v1alpha1.ChatAppService/StreamEvents":      EffectRead,
	"/hawkr.chat.v1alpha1.ChatAppService/StreamStoreEvents": EffectRead,

	// SellerReportAppService
	"/hawkr.seller.v1alpha1.SellerReportAppService/CreateReport":        EffectWrite,
	"/hawkr.seller.v1alpha1.SellerReportAppService/GetMyReport":         EffectRead,
	"/hawkr.seller.v1alpha1.SellerReportAppService/AddReportSupplement": EffectWrite,

	// SocialDelegatedService：憑證是代行 session，不讀帳號，停權的帳號不受影響；宣告仍要誠實標示方法的性質。
	"/hawkr.social.v1alpha1.SocialDelegatedService/GetStoreSocialSummary": EffectRead,
	"/hawkr.social.v1alpha1.SocialDelegatedService/ListStorePosts":        EffectRead,
	"/hawkr.social.v1alpha1.SocialDelegatedService/GetStorePost":          EffectRead,
	"/hawkr.social.v1alpha1.SocialDelegatedService/CreateStorePost":       EffectWrite,
	"/hawkr.social.v1alpha1.SocialDelegatedService/UpdateStorePost":       EffectWrite,
	"/hawkr.social.v1alpha1.SocialDelegatedService/PublishStorePost":      EffectWrite,
	"/hawkr.social.v1alpha1.SocialDelegatedService/UnpublishStorePost":    EffectWrite,
	"/hawkr.social.v1alpha1.SocialDelegatedService/ScheduleStorePost":     EffectWrite,

	// ContentPublicService
	"/hawkr.content.v1alpha1.ContentPublicService/ListLiveBanners": EffectRead,
	"/hawkr.content.v1alpha1.ContentPublicService/CheckAppVersion": EffectRead,

	// SellerAppService
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
	"/hawkr.seller.v1alpha1.SellerAppService/CreatePaymentOnboardingSession": EffectWrite,
	"/hawkr.seller.v1alpha1.SellerAppService/LookupBusinessRegistration":     EffectRead,
	"/hawkr.seller.v1alpha1.SellerAppService/ListMyMeasures":                 EffectRead,
	"/hawkr.seller.v1alpha1.SellerAppService/GetMyMeasure":                   EffectRead,
	"/hawkr.seller.v1alpha1.SellerAppService/SubmitAppeal":                   EffectWrite,
	"/hawkr.seller.v1alpha1.SellerAppService/RequestRemediation":             EffectWrite,
	"/hawkr.seller.v1alpha1.SellerAppService/ListMyInquiries":                EffectRead,
	"/hawkr.seller.v1alpha1.SellerAppService/GetMyInquiry":                   EffectRead,
	"/hawkr.seller.v1alpha1.SellerAppService/ReplyToInquiry":                 EffectWrite,
	// 店家封鎖是保護動作（同 BuyerAppService 的個人封鎖）；名單是讀取。
	"/hawkr.seller.v1alpha1.SellerAppService/BlockBuyer":        EffectSelfService,
	"/hawkr.seller.v1alpha1.SellerAppService/UnblockBuyer":      EffectSelfService,
	"/hawkr.seller.v1alpha1.SellerAppService/ListBlockedBuyers": EffectRead,

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
