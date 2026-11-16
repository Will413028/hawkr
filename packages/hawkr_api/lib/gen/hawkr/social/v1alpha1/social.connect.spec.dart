//
//  Generated code. Do not modify.
//  source: hawkr/social/v1alpha1/social.proto
//

import "package:connectrpc/connect.dart" as connect;
import "social.pb.dart" as hawkrsocialv1alpha1social;

/// SocialPublicService 是任何人都能讀的貼文與留言，不需要登入。
abstract final class SocialPublicService {
  /// Fully-qualified name of the SocialPublicService service.
  static const name = 'hawkr.social.v1alpha1.SocialPublicService';

  /// GetStorePost 讀一篇觀看者可讀的貼文：已發布、沒有被下架或平台隱藏，而且店家不是被觀看者個人封鎖的。其他一律回 not_found，
  /// 不區分原因。店主讀自己店的貼文不套個人封鎖。這個方法不計瀏覽數，瀏覽由 RecordPostView 另外記錄。
  static const getPost = connect.Spec(
    '/$name/GetPost',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.GetPostRequest.new,
    hawkrsocialv1alpha1social.GetPostResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// ListStorePosts 列出觀看者可讀的貼文，依第一次發布的時間新到舊（同一時間以 ID 排序）。不帶篩選是全站動態牆，帶 store_id 是該店的貼文頁，
  /// 帶 hashtag 是該標籤的貼文頁；兩個篩選不能同時帶（invalid_argument）。
  /// 觀看者個人封鎖的店不出現在動態牆，過濾在分頁之前，所以一頁可以少於 page_size，結束只看 next_page_token。
  static const listPosts = connect.Spec(
    '/$name/ListPosts',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.ListPostsRequest.new,
    hawkrsocialv1alpha1social.ListPostsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// ListComments 列出貼文的頂層留言，或某則頂層留言的回覆，依建立時間舊到新。刪除或隱藏而底下有可見回覆的頂層留言是佔位項目
  /// （只有 id、post_id、create_time、reply_count 與 state）；與觀看者任一方向有個人封鎖的買家的留言連同回覆整則不出現。
  /// 貼文讀不到時回 not_found。
  static const listComments = connect.Spec(
    '/$name/ListComments',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.ListCommentsRequest.new,
    hawkrsocialv1alpha1social.ListCommentsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// SuggestTrendingPosts 是「本週主題」：最近發布的 30 篇觀看者可讀的貼文中，瀏覽數最高的前 max_results 篇（同分以發布時間、ID 排序）。
  /// 沒有分頁，時間窗不是日期而是「最近 30 篇」，所以回傳的篇數不超過 30，也可能更少。
  static const suggestTrendingPosts = connect.Spec(
    '/$name/SuggestTrendingPosts',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.SuggestTrendingPostsRequest.new,
    hawkrsocialv1alpha1social.SuggestTrendingPostsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// SuggestTags 是標籤輸入時的建議：公開貼文上名稱符合的標籤與貼文數，貼文數多的在前。全域彙總，不依觀看者。沒有分頁，最多 20 筆。
  static const suggestTags = connect.Spec(
    '/$name/SuggestTags',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.SuggestTagsRequest.new,
    hawkrsocialv1alpha1social.SuggestTagsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// SuggestTrendingTags 是熱門標籤：發布時間落在時間窗內的公開貼文上，各標籤的貼文數，多的在前。全域彙總，不依觀看者。
  /// 這是只取前 max_results 名的排行榜（最多 50 名），不分頁；時間窗最長 30 天。
  static const suggestTrendingTags = connect.Spec(
    '/$name/SuggestTrendingTags',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.SuggestTrendingTagsRequest.new,
    hawkrsocialv1alpha1social.SuggestTrendingTagsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// ListForumEntries 列出店家討論區的頂層留言，或某則頂層留言的回覆，依建立時間舊到新。刪除或隱藏而底下有可見回覆的頂層留言是佔位項目
  /// （只有 id、store_id、create_time、reply_count 與 state）；觀看者封鎖了這家店、或與作者任一方向有個人封鎖時，對應的內容不出現，
  /// 該頁因此可以少於 page_size。店家讀不到時是空的一頁；指定的 parent_id 不屬於這家店回 not_found。
  static const listForumEntries = connect.Spec(
    '/$name/ListForumEntries',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.ListForumEntriesRequest.new,
    hawkrsocialv1alpha1social.ListForumEntriesResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// RecordPostView 記一次貼文瀏覽，匿名也能呼叫。只計觀看者讀得到的貼文，同一篇、同一個台北日、同一個觀看者只算一次；
  /// 讀不到的貼文與沒有計入的呼叫都回成功，不區分原因。登入者以帳號去重，訪客以來源 IP 與 User-Agent 去重；停權的帳號與無效的憑證以訪客計入。
  /// 店主看自己店的貼文不計。每個來源 IP 有額度，超過回 resource_exhausted 與 RetryInfo。瀏覽是寫入，所以不標 NO_SIDE_EFFECTS，client 以 POST 呼叫；
  /// 讀取貼文的方法（GetPost）不計瀏覽。
  static const recordPostView = connect.Spec(
    '/$name/RecordPostView',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.RecordPostViewRequest.new,
    hawkrsocialv1alpha1social.RecordPostViewResponse.new,
  );
}
/// SocialAppService 是登入者在貼文、留言與店家討論區上的互動寫入。呼叫者的身分由伺服器從 session 推導：呼叫者此刻的店擁有該貼文或討論區時，
/// 互動者是店家，否則是買家。被店家封鎖的買家在這家店的留言、發言、編輯與按讚被拒（permission_denied，不另給原因，與讀取回應的
/// viewer_can_comment 一致）。
abstract final class SocialAppService {
  /// Fully-qualified name of the SocialAppService service.
  static const name = 'hawkr.social.v1alpha1.SocialAppService';

  /// SharePost 記一次分享：同一位買家對同一篇只算一次，重送安全。貼文讀不到回 not_found。
  static const sharePost = connect.Spec(
    '/$name/SharePost',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.SharePostRequest.new,
    hawkrsocialv1alpha1social.SharePostResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// LikePost 對貼文按讚，已經讚過也回成功（設定而不是切換）。只有買家能按讚；貼文讀不到回 not_found。
  static const likePost = connect.Spec(
    '/$name/LikePost',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.LikePostRequest.new,
    hawkrsocialv1alpha1social.LikePostResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// UnlikePost 取消貼文的讚，沒有讚過也回成功。
  static const unlikePost = connect.Spec(
    '/$name/UnlikePost',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.UnlikePostRequest.new,
    hawkrsocialv1alpha1social.UnlikePostResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// LikeComment 對留言按讚，已經讚過也回成功。留言讀不到回 not_found。
  static const likeComment = connect.Spec(
    '/$name/LikeComment',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.LikeCommentRequest.new,
    hawkrsocialv1alpha1social.LikeCommentResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// UnlikeComment 取消留言的讚，沒有讚過也回成功。
  static const unlikeComment = connect.Spec(
    '/$name/UnlikeComment',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.UnlikeCommentRequest.new,
    hawkrsocialv1alpha1social.UnlikeCommentResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// CreateComment 在貼文下留言，帶 parent_id 是回覆（回覆只限貼文所屬的店家）。帶 request_id 重送回目前的留言（狀態與計數是此刻的），不重複建立。
  static const createComment = connect.Spec(
    '/$name/CreateComment',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.CreateCommentRequest.new,
    hawkrsocialv1alpha1social.CreateCommentResponse.new,
  );

  /// UpdateComment 修改自己的留言內文（update_mask 列 body）；etag 必填，與目前版本不符回 aborted 與 VERSION_CONFLICT，不寫入。被隱藏的留言不能修改（failed_precondition），已刪除的回 not_found。
  static const updateComment = connect.Spec(
    '/$name/UpdateComment',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.UpdateCommentRequest.new,
    hawkrsocialv1alpha1social.UpdateCommentResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// DeleteComment 刪除留言：作者，或貼文所屬的店家。已經刪除的也回成功；帶 etag 時與目前版本不符回 aborted 與 VERSION_CONFLICT。
  static const deleteComment = connect.Spec(
    '/$name/DeleteComment',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.DeleteCommentRequest.new,
    hawkrsocialv1alpha1social.DeleteCommentResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// CreateForumEntry 在店家的討論區發言，帶 parent_id 是回覆（討論區的回覆不限身分，買家也能回覆）。帶 request_id 重送回目前的項目，不重複建立。
  static const createForumEntry = connect.Spec(
    '/$name/CreateForumEntry',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.CreateForumEntryRequest.new,
    hawkrsocialv1alpha1social.CreateForumEntryResponse.new,
  );

  /// UpdateForumEntry 修改自己的討論區發言內文（update_mask 列 body）；etag 必填，與目前版本不符回 aborted 與 VERSION_CONFLICT，不寫入。被隱藏的不能修改（failed_precondition），已刪除的回 not_found。
  static const updateForumEntry = connect.Spec(
    '/$name/UpdateForumEntry',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.UpdateForumEntryRequest.new,
    hawkrsocialv1alpha1social.UpdateForumEntryResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// DeleteForumEntry 刪除討論區發言：作者，或該店家。已經刪除的也回成功；帶 etag 時與目前版本不符回 aborted 與 VERSION_CONFLICT。
  static const deleteForumEntry = connect.Spec(
    '/$name/DeleteForumEntry',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.DeleteForumEntryRequest.new,
    hawkrsocialv1alpha1social.DeleteForumEntryResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// CreateStorePost 以呼叫者此刻的店建立貼文（店家由伺服器推導，請求不帶 store_id）。publish 為真時同一個動作發布，公開要件（恰好一張圖、內文必填）在轉為公開時驗證；
  /// 草稿可以不完整。帶 request_id 重送回第一次建立的貼文目前的樣子（狀態與計數是此刻的），不重複建立；內容不同回 invalid_argument。需要賣家能力 CAP_MARKETING。
  static const createPost = connect.Spec(
    '/$name/CreatePost',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.CreatePostRequest.new,
    hawkrsocialv1alpha1social.CreatePostResponse.new,
  );

  /// GetMyPost 讀呼叫者此刻的店自己的貼文：任何狀態（草稿、已發布、已下架），不含已刪除；別家的貼文與不存在一樣回 not_found。回應帶 status 與 etag。
  /// 賣家能力 CAP_CONTENT_CONTRACT（停權時仍可讀，才能下架與刪除）。
  static const getMyPost = connect.Spec(
    '/$name/GetMyPost',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.GetMyPostRequest.new,
    hawkrsocialv1alpha1social.GetMyPostResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// ListMyPosts 列出呼叫者此刻的店自己的貼文（含草稿與下架），依建立時間新到舊（同一時間以 ID 排序）；可依 status 篩選。賣家能力 CAP_CONTENT_CONTRACT。
  static const listMyPosts = connect.Spec(
    '/$name/ListMyPosts',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.ListMyPostsRequest.new,
    hawkrsocialv1alpha1social.ListMyPostsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// UpdateStorePost 修改自己的貼文（update_mask 列 body、image_staged_token、product_id、hashtag_inputs 的子集）；etag 必填，與目前版本不符回 aborted 與 VERSION_CONFLICT，不寫入。
  /// 已發布或有待發布排程的貼文被修改後仍要滿足公開要件。需要賣家能力 CAP_MARKETING。
  static const updatePost = connect.Spec(
    '/$name/UpdatePost',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.UpdatePostRequest.new,
    hawkrsocialv1alpha1social.UpdatePostResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// PublishPost 發布草稿或下架的貼文；etag 必填。被內容下架的貼文一律回 failed_precondition，已經是已發布也一樣；沒被下架而已經是已發布時是冪等的 no-op（不檢查 etag、不改版本）。貼文沒有圖片或內文（不滿足公開要件）回 failed_precondition。需要賣家能力 CAP_MARKETING。
  static const publishPost = connect.Spec(
    '/$name/PublishPost',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.PublishPostRequest.new,
    hawkrsocialv1alpha1social.PublishPostResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// UnpublishStorePost 下架已發布的貼文；etag 必填。已經下架的是冪等的 no-op；草稿沒有發布過，回 failed_precondition。收縮類動作，店家停權時仍可執行（CAP_CONTENT_CONTRACT）；帳號停權照動作性質擋寫入（ACCOUNT_SUSPENDED）。
  static const unpublishPost = connect.Spec(
    '/$name/UnpublishPost',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.UnpublishPostRequest.new,
    hawkrsocialv1alpha1social.UnpublishPostResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// DeletePost 刪除自己的貼文；已經刪除的也回成功；帶 etag 時與目前版本不符回 aborted 與 VERSION_CONFLICT。收縮類動作，店家停權時仍可執行（CAP_CONTENT_CONTRACT）；帳號停權照動作性質擋寫入（ACCOUNT_SUSPENDED）。
  static const deletePost = connect.Spec(
    '/$name/DeletePost',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.DeletePostRequest.new,
    hawkrsocialv1alpha1social.DeletePostResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// ScheduleStorePost 排程發布：貼文要是草稿或已下架，publish_time 距現在 5 分鐘到 90 天，而且貼文要滿足公開要件（沒有圖片或內文回 failed_precondition）；貼文已有待發布的排程時，舊的被取代。etag 必填。
  /// 需要賣家能力 CAP_MARKETING。排程的讀取與取消見 ListPendingSchedules、CancelPostSchedule。
  static const schedulePost = connect.Spec(
    '/$name/SchedulePost',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.SchedulePostRequest.new,
    hawkrsocialv1alpha1social.SchedulePostResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// ListPendingSchedules 列出呼叫者此刻的店待發布的排程（含店主自己排的與代行營運排的），發布時間早的在前（同一時間以 ID 排序）。店主看得到代行排的貼文並能取消（CancelPostSchedule）。
  /// 只讀自己店的排程，店家由伺服器推導。
  static const listPendingSchedules = connect.Spec(
    '/$name/ListPendingSchedules',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.ListPendingSchedulesRequest.new,
    hawkrsocialv1alpha1social.ListPendingSchedulesResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// CancelPostSchedule 店主取消貼文待發布的排程（不分發起者；取消原因記為店主取消）；etag 必填，與目前版本不符回 aborted 與 VERSION_CONFLICT。貼文沒有待發布的排程時什麼都不做（不檢查 etag），回目前的 etag。
  /// 貼文不是這家店的回 not_found。需要賣家能力 CAP_MARKETING。
  static const cancelPostSchedule = connect.Spec(
    '/$name/CancelPostSchedule',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.CancelPostScheduleRequest.new,
    hawkrsocialv1alpha1social.CancelPostScheduleResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// ListStoreComments 列出呼叫者此刻的店所有貼文（含草稿與下架的貼文）的頂層留言，依建立時間舊到新；post_id 選填，設了只列那一篇。
  /// 刪除或隱藏而底下有可見回覆的頂層留言是佔位項目。unreplied 為真時只列「未回覆」的留言：本店貼文上、作者不是本店的頂層留言，沒有刪除、沒有被隱藏，
  /// 而且底下沒有任何一則仍可見、作者是本店的回覆（已刪除或被隱藏的店家回覆不算回覆）；此時沒有佔位項目。賣家能力 CAP_CONTENT_CONTRACT。
  static const listStoreComments = connect.Spec(
    '/$name/ListStoreComments',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.ListStoreCommentsRequest.new,
    hawkrsocialv1alpha1social.ListStoreCommentsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// GetStoreSocialSummary 是賣家中心的動態數：呼叫者此刻的店的貼文依狀態的篇數（已刪除的不算），以及待發布排程的篇數。數字由伺服器以資料庫計數，
  /// 不是 client 以列表長度推算。賣家能力 CAP_CONTENT_CONTRACT。
  static const getStoreSocialSummary = connect.Spec(
    '/$name/GetStoreSocialSummary',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.GetStoreSocialSummaryRequest.new,
    hawkrsocialv1alpha1social.GetStoreSocialSummaryResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// ListMySanctions 列出呼叫者（買家）收到的社群處分，新到舊：種類、政策分類與理由，作者有權讀自己的處分。停權的帳號也可以呼叫。
  static const listMySanctions = connect.Spec(
    '/$name/ListMySanctions',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.ListMySanctionsRequest.new,
    hawkrsocialv1alpha1social.ListMySanctionsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// ListMyHiddenContent 列出呼叫者（買家）被平台隱藏的留言與討論區留言，新隱藏到舊：連同本文與隱藏的政策分類與理由（別人看不到被隱藏的內容）。
  /// 已還原的隱藏不在名單裡。停權的帳號也可以呼叫。
  static const listMyHiddenContent = connect.Spec(
    '/$name/ListMyHiddenContent',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.ListMyHiddenContentRequest.new,
    hawkrsocialv1alpha1social.ListMyHiddenContentResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );
}
/// SocialDelegatedService 是代行營運中的平台管理員讀寫代行店家的貼文：憑證是 X-HK-Operation-Session 標頭裡的代行 session（由 website 的 BFF 轉送，不放在請求訊息），
/// 不讀顧客或管理員的身分；目標店家是 session 建立當下綁定的那一家，請求不帶 store_id。商品與訂單由各自模組的 `<模組>DelegatedService` 在 A10 提供。
/// 回應一律不可快取（private, no-store）。
/// 能力（posts.read、posts.write、posts.publish）依方法宣告：憑證缺少或無效回 unauthenticated；session 已結束、到期或撤銷回 failed_precondition；
/// session 沒有授予對應的能力回 permission_denied；店家此刻不可代行（不存在、已關閉、等待刪除、停權）回 failed_precondition，只拒絕、不撤銷 session。
/// 稽核：成功的寫入在同一個 transaction 內留下一列 SUCCEEDED（寫入失敗時一併 rollback）；被拒絕的請求（含讀取）與寫入的業務失敗另開 transaction 留下一列 FAILED；
/// 成功的讀取不稽核。
abstract final class SocialDelegatedService {
  /// Fully-qualified name of the SocialDelegatedService service.
  static const name = 'hawkr.social.v1alpha1.SocialDelegatedService';

  /// ListStorePosts 列出代行店家自己的貼文（含草稿與下架，不含已刪除），依建立時間新到舊（同一時間以 ID 排序）；可依 status 篩選。需要能力 posts.read。
  static const listStorePosts = connect.Spec(
    '/$name/ListStorePosts',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.ListStorePostsRequest.new,
    hawkrsocialv1alpha1social.ListStorePostsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// GetStorePost 讀代行店家的一篇貼文（任何狀態，不含已刪除）；別家的貼文與不存在一樣回 not_found。回應帶 status 與 etag。需要能力 posts.read。
  static const getStorePost = connect.Spec(
    '/$name/GetStorePost',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.GetStorePostRequest.new,
    hawkrsocialv1alpha1social.GetStorePostResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// CreateStorePost 以代行店家建立草稿貼文（建立一律是草稿；要發布再呼叫 PublishStorePost）。同一個 session 以同一個 request_id 重送回第一次建立的貼文目前的樣子，
  /// 不重複建立、不重複寫稽核；內容不同回 failed_precondition。需要能力 posts.write。
  static const createStorePost = connect.Spec(
    '/$name/CreateStorePost',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.CreateStorePostRequest.new,
    hawkrsocialv1alpha1social.CreateStorePostResponse.new,
  );

  /// UpdateStorePost 修改代行店家的貼文（update_mask 列 body、image_staged_token、product_id、hashtag_inputs 的子集）；etag 必填，與目前版本不符回 aborted 與
  /// VERSION_CONFLICT，不寫入也不留稽核。重送規則同 CreateStorePost。需要能力 posts.write。
  static const updateStorePost = connect.Spec(
    '/$name/UpdateStorePost',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.UpdateStorePostRequest.new,
    hawkrsocialv1alpha1social.UpdateStorePostResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// PublishStorePost 立即發布草稿或下架的貼文；etag 必填。重送規則同 CreateStorePost。需要能力 posts.publish。
  static const publishStorePost = connect.Spec(
    '/$name/PublishStorePost',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.PublishStorePostRequest.new,
    hawkrsocialv1alpha1social.PublishStorePostResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// UnpublishStorePost 下架已發布的貼文；etag 必填。重送規則同 CreateStorePost。需要能力 posts.publish。
  static const unpublishStorePost = connect.Spec(
    '/$name/UnpublishStorePost',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.UnpublishStorePostRequest.new,
    hawkrsocialv1alpha1social.UnpublishStorePostResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// ScheduleStorePost 排程發布：貼文要是草稿或已下架，publish_time 距現在 5 分鐘到 90 天；貼文已有待發布的排程時，舊的被取代。排程屬於店家，session 結束後仍會
  /// 在到點時重驗授權並發布（授權已撤回則取消）。稽核寫 POST_SCHEDULE，到點發布時另寫一列 POST_PUBLISH。重送規則同 CreateStorePost。需要能力 posts.publish。
  static const scheduleStorePost = connect.Spec(
    '/$name/ScheduleStorePost',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.ScheduleStorePostRequest.new,
    hawkrsocialv1alpha1social.ScheduleStorePostResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );
}
