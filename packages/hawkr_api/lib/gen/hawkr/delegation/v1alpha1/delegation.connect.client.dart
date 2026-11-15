//
//  Generated code. Do not modify.
//  source: hawkr/delegation/v1alpha1/delegation.proto
//

import "package:connectrpc/connect.dart" as connect;
import "delegation.pb.dart" as hawkrdelegationv1alpha1delegation;
import "delegation.connect.spec.dart" as specs;

/// DelegationDelegatedService 是代行營運中的平台管理員：憑證是 X-HK-Operation-Session 標頭裡的代行 session，不讀顧客或管理員的身分。
/// 回應一律不可快取（private, no-store）。目前只有貼文的方法；商品與訂單在 A10 加進這個 service。
extension type DelegationDelegatedServiceClient (connect.Transport _transport) {
  /// ListPosts 列出代行店家自己的貼文（含草稿與下架，不含已刪除），依建立時間新到舊（同一時間以 ID 排序）；可依 status 篩選。需要能力 posts.read。
  Future<hawkrdelegationv1alpha1delegation.ListPostsResponse> listPosts(
    hawkrdelegationv1alpha1delegation.ListPostsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.DelegationDelegatedService.listPosts,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// GetPost 讀代行店家的一篇貼文（任何狀態，不含已刪除）；別家的貼文與不存在一樣回 not_found。回應帶 status 與 etag。需要能力 posts.read。
  Future<hawkrdelegationv1alpha1delegation.GetPostResponse> getPost(
    hawkrdelegationv1alpha1delegation.GetPostRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.DelegationDelegatedService.getPost,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// CreatePost 以代行店家建立草稿貼文（建立一律是草稿；要發布再呼叫 PublishPost）。同一個 session 以同一個 request_id 重送回第一次建立的貼文目前的樣子，
  /// 不重複建立、不重複寫稽核；內容不同回 failed_precondition。需要能力 posts.write。
  Future<hawkrdelegationv1alpha1delegation.CreatePostResponse> createPost(
    hawkrdelegationv1alpha1delegation.CreatePostRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.DelegationDelegatedService.createPost,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// UpdatePost 修改代行店家的貼文（update_mask 列 body、image_staged_token、product_id、hashtag_inputs 的子集）；etag 必填，與目前版本不符回 aborted 與
  /// VERSION_CONFLICT，不寫入也不留稽核。重送規則同 CreatePost。需要能力 posts.write。
  Future<hawkrdelegationv1alpha1delegation.UpdatePostResponse> updatePost(
    hawkrdelegationv1alpha1delegation.UpdatePostRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.DelegationDelegatedService.updatePost,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// PublishPost 立即發布草稿或下架的貼文；etag 必填。重送規則同 CreatePost。需要能力 posts.publish。
  Future<hawkrdelegationv1alpha1delegation.PublishPostResponse> publishPost(
    hawkrdelegationv1alpha1delegation.PublishPostRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.DelegationDelegatedService.publishPost,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// UnpublishPost 下架已發布的貼文；etag 必填。重送規則同 CreatePost。需要能力 posts.publish。
  Future<hawkrdelegationv1alpha1delegation.UnpublishPostResponse> unpublishPost(
    hawkrdelegationv1alpha1delegation.UnpublishPostRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.DelegationDelegatedService.unpublishPost,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// SchedulePost 排程發布：貼文要是草稿或已下架，publish_time 距現在 5 分鐘到 90 天；貼文已有待發布的排程時，舊的被取代。排程屬於店家，session 結束後仍會
  /// 在到點時重驗授權並發布（授權已撤回則取消）。稽核寫 POST_SCHEDULE，到點發布時另寫一列 POST_PUBLISH。重送規則同 CreatePost。需要能力 posts.publish。
  Future<hawkrdelegationv1alpha1delegation.SchedulePostResponse> schedulePost(
    hawkrdelegationv1alpha1delegation.SchedulePostRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.DelegationDelegatedService.schedulePost,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }
}
