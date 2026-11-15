//
//  Generated code. Do not modify.
//  source: hawkr/delegation/v1alpha1/delegation.proto
//

import "package:connectrpc/connect.dart" as connect;
import "delegation.pb.dart" as hawkrdelegationv1alpha1delegation;

/// DelegationDelegatedService 是代行營運中的平台管理員：憑證是 X-HK-Operation-Session 標頭裡的代行 session，不讀顧客或管理員的身分。
/// 回應一律不可快取（private, no-store）。目前只有貼文的方法；商品與訂單在 A10 加進這個 service。
abstract final class DelegationDelegatedService {
  /// Fully-qualified name of the DelegationDelegatedService service.
  static const name = 'hawkr.delegation.v1alpha1.DelegationDelegatedService';

  /// ListPosts 列出代行店家自己的貼文（含草稿與下架，不含已刪除），依建立時間新到舊（同一時間以 ID 排序）；可依 status 篩選。需要能力 posts.read。
  static const listPosts = connect.Spec(
    '/$name/ListPosts',
    connect.StreamType.unary,
    hawkrdelegationv1alpha1delegation.ListPostsRequest.new,
    hawkrdelegationv1alpha1delegation.ListPostsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// GetPost 讀代行店家的一篇貼文（任何狀態，不含已刪除）；別家的貼文與不存在一樣回 not_found。回應帶 status 與 etag。需要能力 posts.read。
  static const getPost = connect.Spec(
    '/$name/GetPost',
    connect.StreamType.unary,
    hawkrdelegationv1alpha1delegation.GetPostRequest.new,
    hawkrdelegationv1alpha1delegation.GetPostResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// CreatePost 以代行店家建立草稿貼文（建立一律是草稿；要發布再呼叫 PublishPost）。同一個 session 以同一個 request_id 重送回第一次建立的貼文目前的樣子，
  /// 不重複建立、不重複寫稽核；內容不同回 failed_precondition。需要能力 posts.write。
  static const createPost = connect.Spec(
    '/$name/CreatePost',
    connect.StreamType.unary,
    hawkrdelegationv1alpha1delegation.CreatePostRequest.new,
    hawkrdelegationv1alpha1delegation.CreatePostResponse.new,
  );

  /// UpdatePost 修改代行店家的貼文（update_mask 列 body、image_staged_token、product_id、hashtag_inputs 的子集）；etag 必填，與目前版本不符回 aborted 與
  /// VERSION_CONFLICT，不寫入也不留稽核。重送規則同 CreatePost。需要能力 posts.write。
  static const updatePost = connect.Spec(
    '/$name/UpdatePost',
    connect.StreamType.unary,
    hawkrdelegationv1alpha1delegation.UpdatePostRequest.new,
    hawkrdelegationv1alpha1delegation.UpdatePostResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// PublishPost 立即發布草稿或下架的貼文；etag 必填。重送規則同 CreatePost。需要能力 posts.publish。
  static const publishPost = connect.Spec(
    '/$name/PublishPost',
    connect.StreamType.unary,
    hawkrdelegationv1alpha1delegation.PublishPostRequest.new,
    hawkrdelegationv1alpha1delegation.PublishPostResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// UnpublishPost 下架已發布的貼文；etag 必填。重送規則同 CreatePost。需要能力 posts.publish。
  static const unpublishPost = connect.Spec(
    '/$name/UnpublishPost',
    connect.StreamType.unary,
    hawkrdelegationv1alpha1delegation.UnpublishPostRequest.new,
    hawkrdelegationv1alpha1delegation.UnpublishPostResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// SchedulePost 排程發布：貼文要是草稿或已下架，publish_time 距現在 5 分鐘到 90 天；貼文已有待發布的排程時，舊的被取代。排程屬於店家，session 結束後仍會
  /// 在到點時重驗授權並發布（授權已撤回則取消）。稽核寫 POST_SCHEDULE，到點發布時另寫一列 POST_PUBLISH。重送規則同 CreatePost。需要能力 posts.publish。
  static const schedulePost = connect.Spec(
    '/$name/SchedulePost',
    connect.StreamType.unary,
    hawkrdelegationv1alpha1delegation.SchedulePostRequest.new,
    hawkrdelegationv1alpha1delegation.SchedulePostResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );
}
