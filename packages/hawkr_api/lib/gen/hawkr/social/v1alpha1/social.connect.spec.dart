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

  /// GetPost 讀一篇觀看者可讀的貼文：已發布、沒有被下架或平台隱藏，而且店家不是被觀看者個人封鎖的。其他一律回 not_found，
  /// 不區分原因。店主讀自己店的貼文不套個人封鎖。這個方法不計瀏覽數，瀏覽由 RecordPostView 另外記錄。
  static const getPost = connect.Spec(
    '/$name/GetPost',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.GetPostRequest.new,
    hawkrsocialv1alpha1social.GetPostResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// ListPosts 列出觀看者可讀的貼文，依第一次發布的時間新到舊（同一時間以 ID 排序）。不帶篩選是全站動態牆，帶 store_id 是該店的貼文頁，
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

  /// CreateComment 在貼文下留言，帶 parent_id 是回覆（回覆只限貼文所屬的店家）。帶 request_id 重送回第一次建立的留言，不重複建立。
  static const createComment = connect.Spec(
    '/$name/CreateComment',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.CreateCommentRequest.new,
    hawkrsocialv1alpha1social.CreateCommentResponse.new,
  );

  /// UpdateComment 修改自己的留言內文；被隱藏的留言不能修改（failed_precondition），已刪除的回 not_found。
  static const updateComment = connect.Spec(
    '/$name/UpdateComment',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.UpdateCommentRequest.new,
    hawkrsocialv1alpha1social.UpdateCommentResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// DeleteComment 刪除留言：作者，或貼文所屬的店家。已經刪除的也回成功。
  static const deleteComment = connect.Spec(
    '/$name/DeleteComment',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.DeleteCommentRequest.new,
    hawkrsocialv1alpha1social.DeleteCommentResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// CreateForumEntry 在店家的討論區發言，帶 parent_id 是回覆（回覆只限該店家）。帶 request_id 重送回第一次建立的項目，不重複建立。
  static const createForumEntry = connect.Spec(
    '/$name/CreateForumEntry',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.CreateForumEntryRequest.new,
    hawkrsocialv1alpha1social.CreateForumEntryResponse.new,
  );

  /// UpdateForumEntry 修改自己的討論區發言內文；被隱藏的不能修改（failed_precondition），已刪除的回 not_found。
  static const updateForumEntry = connect.Spec(
    '/$name/UpdateForumEntry',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.UpdateForumEntryRequest.new,
    hawkrsocialv1alpha1social.UpdateForumEntryResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// DeleteForumEntry 刪除討論區發言：作者，或該店家。已經刪除的也回成功。
  static const deleteForumEntry = connect.Spec(
    '/$name/DeleteForumEntry',
    connect.StreamType.unary,
    hawkrsocialv1alpha1social.DeleteForumEntryRequest.new,
    hawkrsocialv1alpha1social.DeleteForumEntryResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );
}
