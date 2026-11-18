import 'dart:async';

import 'package:fixnum/fixnum.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/image.pb.dart' as common;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pb.dart'
    as common_store;
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart' show ConnectException;
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

/// 測試用的社群資料與 fake client。能不能刪除、回覆、留言都由呼叫端明確指定（伺服器的 `viewer_can_*`）。
pb.Comment fakeComment(
  String id, {
  String body = '留言內文',
  String author = '小明',
  bool canDelete = false,
  bool canReply = false,
  int replyCount = 0,
  pb.CommentState state = pb.CommentState.COMMENT_STATE_VISIBLE,
}) => pb.Comment(
  id: id,
  postId: '101',
  state: state,
  author: pb.Actor(
    kind: pb.ActorKind.ACTOR_KIND_BUYER,
    id: 'buyer-$id',
    name: author,
  ),
  body: body,
  replyCount: Int64(replyCount),
  viewerCanDelete: canDelete,
  viewerCanReply: canReply,
  etag: 'etag-$id',
);

pb.ForumEntry fakeForumEntry(
  String id, {
  String body = '討論區內文',
  String author = '小華',
  bool canEdit = false,
  bool canDelete = false,
  bool canReply = false,
  int replyCount = 0,
}) => pb.ForumEntry(
  id: id,
  storeId: '7',
  state: pb.CommentState.COMMENT_STATE_VISIBLE,
  author: pb.Actor(
    kind: pb.ActorKind.ACTOR_KIND_BUYER,
    id: 'buyer-$id',
    name: author,
  ),
  body: body,
  replyCount: Int64(replyCount),
  viewerCanEdit: canEdit,
  viewerCanDelete: canDelete,
  viewerCanReply: canReply,
  etag: 'etag-$id',
);

pb.Post fakePost({
  String id = '101',
  bool viewerCanComment = true,
  pb.Actor? viewerActor,
}) => pb.Post(
  id: id,
  store: common_store.StoreCard(id: '7', storeCode: 'shop', name: '好店'),
  body: '貼文內文',
  image: common.Image(uriTemplate: 'https://img.test/{preset}/p.jpg'),
  viewerCanComment: viewerCanComment,
  viewerActor: viewerActor,
);

pb.Actor storeActor(String name) =>
    pb.Actor(kind: pb.ActorKind.ACTOR_KIND_STORE, id: '7', name: name);

pb.Actor buyerActor(String name) =>
    pb.Actor(kind: pb.ActorKind.ACTOR_KIND_BUYER, id: 'b1', name: name);

/// 公開讀取的 fake。沒給的方法不註冊（呼叫會失敗）。
class FakeSocialPublic {
  FakeSocialPublic({
    this.post,
    this.posts = const [],
    this.comments = const [],
    this.replies = const {},
    this.forumEntries = const [],
    this.forumReplies = const {},
    this.forumViewerCanComment = true,
    this.forumViewerActor,
    this.commentsResponder,
    this.postsResponder,
    this.getPostError,
    this.recordViewError,
  });

  pb.Post? post;
  List<pb.Post> posts;
  List<pb.Comment> comments;
  Map<String, List<pb.Comment>> replies;
  List<pb.ForumEntry> forumEntries;
  Map<String, List<pb.ForumEntry>> forumReplies;
  bool forumViewerCanComment;
  pb.Actor? forumViewerActor;

  /// 設了就由它回 `ListComments`（測分頁與失敗）；沒設時依 [comments]、[replies]。
  pb.ListCommentsResponse Function(pb.ListCommentsRequest request)?
  commentsResponder;

  /// 設了就由它回 `ListPosts`（測分頁與競態）。
  Future<pb.ListPostsResponse> Function(pb.ListPostsRequest request)?
  postsResponder;
  final List<pb.ListCommentsRequest> listCommentsRequests = [];
  final List<pb.ListForumEntriesRequest> listForumRequests = [];
  int listForumCalls = 0;
  ConnectException? getPostError;
  ConnectException? recordViewError;

  int listPostsCalls = 0;
  final List<pb.ListPostsRequest> listPostsRequests = [];
  int getPostCalls = 0;
  int listCommentsCalls = 0;
  int recordViewCalls = 0;

  SocialPublicServiceClient client() => SocialPublicServiceClient(
    FakeTransportBuilder()
        .unary(SocialPublicService.listPosts, (req, _) async {
          listPostsCalls++;
          listPostsRequests.add(req);
          final responder = postsResponder;
          if (responder != null) return responder(req);
          return pb.ListPostsResponse(posts: posts);
        })
        .unary(SocialPublicService.getPost, (req, _) {
          getPostCalls++;
          if (getPostError != null) throw getPostError!;
          return pb.GetPostResponse(post: post ?? fakePost());
        })
        .unary(SocialPublicService.listComments, (req, _) {
          listCommentsCalls++;
          listCommentsRequests.add(req);
          final responder = commentsResponder;
          if (responder != null) return responder(req);
          final parent = req.hasParentId() ? req.parentId : null;
          return pb.ListCommentsResponse(
            comments: parent == null ? comments : (replies[parent] ?? []),
          );
        })
        .unary(SocialPublicService.listForumEntries, (req, _) {
          listForumCalls++;
          listForumRequests.add(req);
          final parent = req.hasParentId() ? req.parentId : null;
          return pb.ListForumEntriesResponse(
            entries: parent == null
                ? forumEntries
                : (forumReplies[parent] ?? []),
            viewerCanComment: forumViewerCanComment,
            viewerActor: forumViewerActor,
          );
        })
        .unary(SocialPublicService.recordPostView, (req, _) {
          recordViewCalls++;
          if (recordViewError != null) throw recordViewError!;
          return pb.RecordPostViewResponse();
        })
        .build(),
  );
}

/// 寫入的 fake；[created] 收到每次 CreateComment 的請求。
class FakeSocialApp {
  FakeSocialApp({this.createError, this.likeError});

  /// 設了就讓貼文的讚與取消讚失敗。
  ConnectException? likeError;
  final List<pb.LikePostRequest> postLikes = [];

  ConnectException? createError;

  /// 設了就讓寫入卡住，直到測試完成它（完成錯誤代表寫入失敗）。
  Completer<void>? pending;
  final List<pb.CreateCommentRequest> created = [];
  final List<pb.DeleteCommentRequest> deleted = [];
  final List<pb.CreateForumEntryRequest> forumCreated = [];
  final List<pb.UpdateForumEntryRequest> forumUpdated = [];
  final List<pb.DeleteForumEntryRequest> forumDeleted = [];

  SocialAppServiceClient client() => SocialAppServiceClient(
    FakeTransportBuilder()
        .unary(SocialAppService.createComment, (req, _) {
          created.add(req);
          if (createError != null) throw createError!;
          return pb.CreateCommentResponse(
            comment: fakeComment('new', body: req.body),
          );
        })
        .unary(SocialAppService.createForumEntry, (req, _) async {
          forumCreated.add(req);
          if (pending != null) await pending!.future;
          if (createError != null) throw createError!;
          return pb.CreateForumEntryResponse(
            entry: fakeForumEntry('new', body: req.body),
          );
        })
        .unary(SocialAppService.updateForumEntry, (req, _) async {
          forumUpdated.add(req);
          if (pending != null) await pending!.future;
          return pb.UpdateForumEntryResponse(entry: req.entry);
        })
        .unary(SocialAppService.deleteForumEntry, (req, _) async {
          forumDeleted.add(req);
          if (pending != null) await pending!.future;
          return pb.DeleteForumEntryResponse();
        })
        .unary(SocialAppService.likePost, (req, _) {
          postLikes.add(req);
          if (likeError != null) throw likeError!;
          return pb.LikePostResponse();
        })
        .unary(SocialAppService.deleteComment, (req, _) {
          deleted.add(req);
          return pb.DeleteCommentResponse();
        })
        .build(),
  );
}

/// 一開始是訪客，[signIn] 之後換成已登入帳號（測打開畫面後才登入）。
class SwitchableSession extends Session {
  @override
  Future<SessionModel?> build() async => null;

  void signIn() => state = const AsyncData(
    SessionModel(userId: 'account-3001', nickname: 'Iris'),
  );
}
