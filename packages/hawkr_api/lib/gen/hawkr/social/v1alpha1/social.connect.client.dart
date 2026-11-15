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

  /// ListTrendingTags 是熱門標籤：發布時間落在時間窗內的公開貼文上，各標籤的貼文數，多的在前。全域彙總，不依觀看者。
  /// 這是只取前 max_results 名的排行榜（最多 50 名），不分頁；時間窗最長 30 天。
  Future<hawkrsocialv1alpha1social.ListTrendingTagsResponse> listTrendingTags(
    hawkrsocialv1alpha1social.ListTrendingTagsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SocialPublicService.listTrendingTags,
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
}
