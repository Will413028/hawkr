//
//  Generated code. Do not modify.
//  source: hawkr/social/v1alpha1/social.proto
//

import "package:connectrpc/connect.dart" as connect;
import "social.pb.dart" as hawkrsocialv1alpha1social;
import "social.connect.spec.dart" as specs;

/// SocialPublicService 是任何人都能讀的貼文與留言，不需要登入。
extension type SocialPublicServiceClient (connect.Transport _transport) {
  /// GetPost 讀一篇觀看者可讀的貼文：已發布、沒有被下架或平台隱藏，而且店家不是被觀看者個人封鎖的。其他一律回 not_found，
  /// 不區分原因。店主讀自己店的貼文不套個人封鎖。這個方法不計瀏覽數，瀏覽由 RecordPostView 另外記錄。
  Future<hawkrsocialv1alpha1social.GetPostResponse> getPost(
    hawkrsocialv1alpha1social.GetPostRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SocialPublicService.getPost,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// ListPosts 列出觀看者可讀的貼文，依第一次發布的時間新到舊（同一時間以 ID 排序）。不帶篩選是全站動態牆，帶 store_id 是該店的貼文頁，
  /// 帶 hashtag 是該標籤的貼文頁；兩個篩選不能同時帶（invalid_argument）。
  /// 觀看者個人封鎖的店不出現在動態牆，過濾在分頁之前，所以一頁可以少於 page_size，結束只看 next_page_token。
  Future<hawkrsocialv1alpha1social.ListPostsResponse> listPosts(
    hawkrsocialv1alpha1social.ListPostsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SocialPublicService.listPosts,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// ListComments 列出貼文的頂層留言，或某則頂層留言的回覆，依建立時間舊到新。刪除或隱藏而底下有可見回覆的頂層留言是佔位項目
  /// （只有 id、post_id、create_time、reply_count 與 state）；與觀看者任一方向有個人封鎖的買家的留言連同回覆整則不出現。
  /// 貼文讀不到時回 not_found。
  Future<hawkrsocialv1alpha1social.ListCommentsResponse> listComments(
    hawkrsocialv1alpha1social.ListCommentsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SocialPublicService.listComments,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// SuggestTrendingPosts 是「本週主題」：最近發布的 30 篇觀看者可讀的貼文中，瀏覽數最高的前 max_results 篇（同分以發布時間、ID 排序）。
  /// 沒有分頁，時間窗不是日期而是「最近 30 篇」，所以回傳的篇數不超過 30，也可能更少。
  Future<hawkrsocialv1alpha1social.SuggestTrendingPostsResponse> suggestTrendingPosts(
    hawkrsocialv1alpha1social.SuggestTrendingPostsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SocialPublicService.suggestTrendingPosts,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// SuggestTags 是標籤輸入時的建議：公開貼文上名稱符合的標籤與貼文數，貼文數多的在前。全域彙總，不依觀看者。沒有分頁，最多 20 筆。
  Future<hawkrsocialv1alpha1social.SuggestTagsResponse> suggestTags(
    hawkrsocialv1alpha1social.SuggestTagsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SocialPublicService.suggestTags,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// SuggestTrendingTags 是熱門標籤：發布時間落在時間窗內的公開貼文上，各標籤的貼文數，多的在前。全域彙總，不依觀看者。
  /// 這是只取前 max_results 名的排行榜（最多 50 名），不分頁；時間窗最長 30 天。
  Future<hawkrsocialv1alpha1social.SuggestTrendingTagsResponse> suggestTrendingTags(
    hawkrsocialv1alpha1social.SuggestTrendingTagsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SocialPublicService.suggestTrendingTags,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// ListForumEntries 列出店家討論區的頂層留言，或某則頂層留言的回覆，依建立時間舊到新。刪除或隱藏而底下有可見回覆的頂層留言是佔位項目
  /// （只有 id、store_id、create_time、reply_count 與 state）；觀看者封鎖了這家店、或與作者任一方向有個人封鎖時，對應的內容不出現，
  /// 該頁因此可以少於 page_size。店家讀不到時是空的一頁；指定的 parent_id 不屬於這家店回 not_found。
  Future<hawkrsocialv1alpha1social.ListForumEntriesResponse> listForumEntries(
    hawkrsocialv1alpha1social.ListForumEntriesRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SocialPublicService.listForumEntries,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// RecordPostView 記一次貼文瀏覽，匿名也能呼叫。只計觀看者讀得到的貼文，同一篇、同一個台北日、同一個觀看者只算一次；
  /// 讀不到的貼文與沒有計入的呼叫都回成功，不區分原因。登入者以帳號去重，訪客以來源 IP 與 User-Agent 去重；停權的帳號與無效的憑證以訪客計入。
  /// 店主看自己店的貼文不計。每個來源 IP 有額度，超過回 resource_exhausted 與 RetryInfo。瀏覽是寫入，所以不標 NO_SIDE_EFFECTS，client 以 POST 呼叫；
  /// 讀取貼文的方法（GetPost）不計瀏覽。
  Future<hawkrsocialv1alpha1social.RecordPostViewResponse> recordPostView(
    hawkrsocialv1alpha1social.RecordPostViewRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SocialPublicService.recordPostView,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }
}
/// SocialAppService 是登入者在貼文、留言與店家討論區上的互動寫入。呼叫者的身分由伺服器從 session 推導：呼叫者此刻的店擁有該貼文或討論區時，
/// 互動者是店家，否則是買家。被店家封鎖的買家在這家店的留言、發言、編輯與按讚被拒（permission_denied，不另給原因，與讀取回應的
/// viewer_can_comment 一致）。
extension type SocialAppServiceClient (connect.Transport _transport) {
  /// SharePost 記一次分享：同一位買家對同一篇只算一次，重送安全。貼文讀不到回 not_found。
  Future<hawkrsocialv1alpha1social.SharePostResponse> sharePost(
    hawkrsocialv1alpha1social.SharePostRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SocialAppService.sharePost,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// LikePost 對貼文按讚，已經讚過也回成功（設定而不是切換）。只有買家能按讚；貼文讀不到回 not_found。
  Future<hawkrsocialv1alpha1social.LikePostResponse> likePost(
    hawkrsocialv1alpha1social.LikePostRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SocialAppService.likePost,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// UnlikePost 取消貼文的讚，沒有讚過也回成功。
  Future<hawkrsocialv1alpha1social.UnlikePostResponse> unlikePost(
    hawkrsocialv1alpha1social.UnlikePostRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SocialAppService.unlikePost,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// LikeComment 對留言按讚，已經讚過也回成功。留言讀不到回 not_found。
  Future<hawkrsocialv1alpha1social.LikeCommentResponse> likeComment(
    hawkrsocialv1alpha1social.LikeCommentRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SocialAppService.likeComment,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// UnlikeComment 取消留言的讚，沒有讚過也回成功。
  Future<hawkrsocialv1alpha1social.UnlikeCommentResponse> unlikeComment(
    hawkrsocialv1alpha1social.UnlikeCommentRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SocialAppService.unlikeComment,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// CreateComment 在貼文下留言，帶 parent_id 是回覆（回覆只限貼文所屬的店家）。帶 request_id 重送回目前的留言（狀態與計數是此刻的），不重複建立。
  Future<hawkrsocialv1alpha1social.CreateCommentResponse> createComment(
    hawkrsocialv1alpha1social.CreateCommentRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SocialAppService.createComment,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// UpdateComment 修改自己的留言內文（update_mask 列 body）；etag 必填，與目前版本不符回 aborted 與 VERSION_CONFLICT，不寫入。被隱藏的留言不能修改（failed_precondition），已刪除的回 not_found。
  Future<hawkrsocialv1alpha1social.UpdateCommentResponse> updateComment(
    hawkrsocialv1alpha1social.UpdateCommentRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SocialAppService.updateComment,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// DeleteComment 刪除留言：作者，或貼文所屬的店家。已經刪除的也回成功；帶 etag 時與目前版本不符回 aborted 與 VERSION_CONFLICT。
  Future<hawkrsocialv1alpha1social.DeleteCommentResponse> deleteComment(
    hawkrsocialv1alpha1social.DeleteCommentRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SocialAppService.deleteComment,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// CreateForumEntry 在店家的討論區發言，帶 parent_id 是回覆（討論區的回覆不限身分，買家也能回覆）。帶 request_id 重送回目前的項目，不重複建立，不重複建立。
  Future<hawkrsocialv1alpha1social.CreateForumEntryResponse> createForumEntry(
    hawkrsocialv1alpha1social.CreateForumEntryRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SocialAppService.createForumEntry,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// UpdateForumEntry 修改自己的討論區發言內文（update_mask 列 body）；etag 必填，與目前版本不符回 aborted 與 VERSION_CONFLICT，不寫入。被隱藏的不能修改（failed_precondition），已刪除的回 not_found。
  Future<hawkrsocialv1alpha1social.UpdateForumEntryResponse> updateForumEntry(
    hawkrsocialv1alpha1social.UpdateForumEntryRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SocialAppService.updateForumEntry,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// DeleteForumEntry 刪除討論區發言：作者，或該店家。已經刪除的也回成功；帶 etag 時與目前版本不符回 aborted 與 VERSION_CONFLICT。
  Future<hawkrsocialv1alpha1social.DeleteForumEntryResponse> deleteForumEntry(
    hawkrsocialv1alpha1social.DeleteForumEntryRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SocialAppService.deleteForumEntry,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }
}
