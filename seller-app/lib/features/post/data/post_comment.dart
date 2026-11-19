import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pbenum.dart';
import 'package:hawkr_api/images.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';

/// 被刪除或被平台隱藏的頂層項目（底下還有可見回覆時才會出現）顯示的佔位文字；
/// 其他狀態是 null（顯示內文）。
String? commentPlaceholderText(CommentState state, UiLocale locale) =>
    state.placeholder(locale);

String _authorName(pb.Actor actor) => actor.hasName() ? actor.name.trim() : '';

String _authorAvatar(pb.Actor actor) =>
    actor.hasAvatar() ? actor.avatar.uri(ImagePreset.avatar) : '';

/// 貼文的一則留言（頂層或回覆）。權限一律取伺服器的 `viewer_can_*`，client 不自己推導。
class PostComment {
  const PostComment({
    required this.id,
    required this.postId,
    required this.state,
    this.parentId,
    this.replyToId,
    this.authorName = '',
    this.authorAvatar = '',
    this.body = '',
    this.createTime,
    this.likeCount = 0,
    this.replyCount = 0,
    this.viewerLiked = false,
    this.isMine = false,
    this.authorIsBuyer = false,
    this.viewerCanEdit = false,
    this.viewerCanDelete = false,
    this.viewerCanReply = false,
    this.etag = '',
  });

  factory PostComment.fromProto(pb.Comment c) => PostComment(
    id: c.id,
    postId: c.postId,
    parentId: c.hasParentId() ? c.parentId : null,
    replyToId: c.hasReplyToId() ? c.replyToId : null,
    state: c.state,
    authorName: _authorName(c.author),
    authorAvatar: _authorAvatar(c.author),
    body: c.body,
    createTime: c.hasCreateTime() ? c.createTime.toDateTime() : null,
    likeCount: c.likeCount.toInt(),
    replyCount: c.replyCount.toInt(),
    viewerLiked: c.viewerLiked,
    isMine: c.isMine,
    authorIsBuyer: c.author.kind == ActorKind.ACTOR_KIND_BUYER,
    viewerCanEdit: c.viewerCanEdit,
    viewerCanDelete: c.viewerCanDelete,
    viewerCanReply: c.viewerCanReply,
    etag: c.etag,
  );

  final String id;
  final String postId;
  final String? parentId;
  final String? replyToId;
  final CommentState state;
  final String authorName;
  final String authorAvatar;
  final String body;
  final DateTime? createTime;
  final int likeCount;
  final int replyCount;
  final bool viewerLiked;
  final bool isMine;
  final bool authorIsBuyer;
  final bool viewerCanEdit;
  final bool viewerCanDelete;
  final bool viewerCanReply;
  final String etag;

  bool get isVisible => state == CommentState.COMMENT_STATE_VISIBLE;

  String? placeholderTextFor(UiLocale locale) =>
      commentPlaceholderText(state, locale);

  PostComment withLiked(bool liked) {
    if (liked == viewerLiked) return this;
    return PostComment(
      id: id,
      postId: postId,
      parentId: parentId,
      replyToId: replyToId,
      state: state,
      authorName: authorName,
      authorAvatar: authorAvatar,
      body: body,
      createTime: createTime,
      likeCount: likeCount + (liked ? 1 : -1),
      replyCount: replyCount,
      viewerLiked: liked,
      isMine: isMine,
      authorIsBuyer: authorIsBuyer,
      viewerCanEdit: viewerCanEdit,
      viewerCanDelete: viewerCanDelete,
      viewerCanReply: viewerCanReply,
      etag: etag,
    );
  }
}

/// 店家討論區的一則發言（頂層或回覆）。討論區沒有按讚。
class StoreForumEntry {
  const StoreForumEntry({
    required this.id,
    required this.storeId,
    required this.state,
    this.parentId,
    this.replyToId,
    this.authorName = '',
    this.authorAvatar = '',
    this.body = '',
    this.createTime,
    this.replyCount = 0,
    this.isMine = false,
    this.authorIsBuyer = false,
    this.viewerCanEdit = false,
    this.viewerCanDelete = false,
    this.viewerCanReply = false,
    this.etag = '',
  });

  factory StoreForumEntry.fromProto(pb.ForumEntry e) => StoreForumEntry(
    id: e.id,
    storeId: e.storeId,
    parentId: e.hasParentId() ? e.parentId : null,
    replyToId: e.hasReplyToId() ? e.replyToId : null,
    state: e.state,
    authorName: _authorName(e.author),
    authorAvatar: _authorAvatar(e.author),
    body: e.body,
    createTime: e.hasCreateTime() ? e.createTime.toDateTime() : null,
    replyCount: e.replyCount.toInt(),
    isMine: e.isMine,
    authorIsBuyer: e.author.kind == ActorKind.ACTOR_KIND_BUYER,
    viewerCanEdit: e.viewerCanEdit,
    viewerCanDelete: e.viewerCanDelete,
    viewerCanReply: e.viewerCanReply,
    etag: e.etag,
  );

  final String id;
  final String storeId;
  final String? parentId;
  final String? replyToId;
  final CommentState state;
  final String authorName;
  final String authorAvatar;
  final String body;
  final DateTime? createTime;
  final int replyCount;
  final bool isMine;
  final bool authorIsBuyer;
  final bool viewerCanEdit;
  final bool viewerCanDelete;
  final bool viewerCanReply;
  final String etag;

  bool get isVisible => state == CommentState.COMMENT_STATE_VISIBLE;

  String? placeholderTextFor(UiLocale locale) =>
      commentPlaceholderText(state, locale);
}
