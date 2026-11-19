import 'dart:async';

import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:uuid/uuid.dart';

import '../../../core/session/viewer_scoped_notifier.dart';
import '../data/social_content_repository.dart';
import 'post_entity_store.dart';
import 'thread_notifier.dart';
import 'thread_tree.dart';
import 'write_result.dart';

part 'comments_notifier.g.dart';

class CommentsState implements ThreadStateView<CommentsState, pb.Comment> {
  CommentsState({
    this.phase = ThreadPhase.loading,
    ThreadTree<pb.Comment>? tree,
    this.nextPageToken = '',
    this.sending = false,
  }) : tree = tree ?? ThreadTree<pb.Comment>(commentCodec);

  @override
  final ThreadPhase phase;
  @override
  final ThreadTree<pb.Comment> tree;
  final String nextPageToken;
  final bool sending;

  CommentsState copyWith({
    ThreadPhase? phase,
    ThreadTree<pb.Comment>? tree,
    String? nextPageToken,
    bool? sending,
  }) => CommentsState(
    phase: phase ?? this.phase,
    tree: tree ?? this.tree,
    nextPageToken: nextPageToken ?? this.nextPageToken,
    sending: sending ?? this.sending,
  );

  @override
  CommentsState withThread({
    ThreadPhase? phase,
    ThreadTree<pb.Comment>? tree,
  }) => copyWith(phase: phase, tree: tree);
}

/// 一則貼文的留言：留言以 id 存一份，回覆以 parentId 分組；讀取、分頁、回覆展開在 [ThreadNotifier]，寫入在這裡。
///
/// 換觀看者時整份重建清空；每個跨 await 的路徑回來先比對世代，過期就丟棄。重讀失敗保留舊值；
/// 留言數的變動寫回貼文實體，寫回只看貼文實體自己的世代，不依賴這個 provider 還活著。
@riverpod
class Comments extends _$Comments
    with ViewerScoped, ThreadNotifier<CommentsState, pb.Comment> {
  static const _uuid = Uuid();

  final Set<String> _likesInFlight = <String>{};

  @override
  CommentsState build(int postId) {
    bindViewer();
    _likesInFlight.clear();
    startThread();
    return CommentsState();
  }

  SocialContentRepository get _repo =>
      ref.read(socialContentRepositoryProvider);

  @override
  Future<ThreadPage<pb.Comment>> fetchPage({
    String? parentId,
    String pageToken = '',
  }) async {
    final response = await _repo.listComments(
      postId.toString(),
      parentId: parentId,
      pageToken: pageToken,
    );
    return ThreadPage(response.comments, response.nextPageToken);
  }

  @override
  CommentsState firstPageState(
    CommentsState current,
    ThreadTree<pb.Comment> tree,
    ThreadPage<pb.Comment> page,
  ) => CommentsState(
    phase: ThreadPhase.ready,
    tree: tree,
    nextPageToken: page.nextPageToken,
    sending: current.sending,
  );

  @override
  String rootsPageToken(CommentsState current) => current.nextPageToken;

  @override
  CommentsState withRootsPageToken(CommentsState current, String token) =>
      current.copyWith(nextPageToken: token);

  /// 使用者要求的重讀（下拉、重試）：留言與貼文實體（留言權限、身分、留言數）一起重讀。
  Future<bool> reload() {
    unawaited(ref.read(postEntityStoreProvider.notifier).refresh(postId));
    return loadFirstPage(beginScope());
  }

  Future<bool> loadMore() => loadMoreRoots();

  /// 新增留言；[parentId] 給了是回覆。成功後加進列表並把貼文的留言數加一。
  Future<WriteResult> create(
    String body, {
    String? parentId,
    String? replyToId,
  }) async {
    if (state.sending) return const WriteResult.discarded();
    final scope = beginScope();
    final store = ref.read(postEntityStoreProvider.notifier);
    final storeScope = store.beginScope();
    state = state.copyWith(sending: true);
    final pb.Comment created;
    try {
      created = await _repo.createComment(
        postId.toString(),
        body,
        parentId: parentId,
        replyToId: replyToId,
        requestId: _uuid.v4(),
      );
    } catch (e) {
      if (!scope.isCurrent) return const WriteResult.discarded();
      state = state.copyWith(sending: false);
      return WriteResult.failed(e);
    }
    if (storeScope.isCurrent) store.adjustCommentCount(postId, 1);
    if (!scope.isCurrent) return const WriteResult.discarded();
    applyCreated(created);
    state = state.copyWith(sending: false);
    return const WriteResult.applied();
  }

  /// 刪除一則留言：先在本地套用，伺服器失敗只還原這一則；觀看者換了就丟棄。
  Future<WriteResult> delete(pb.Comment comment) async {
    final deletion = applyDelete(comment.id);
    if (deletion == null) return const WriteResult.discarded();
    final scope = beginScope();
    final store = ref.read(postEntityStoreProvider.notifier);
    final storeScope = store.beginScope();
    try {
      await _repo.deleteComment(comment);
    } catch (e) {
      if (!scope.isCurrent) return const WriteResult.discarded();
      rollbackDelete(deletion);
      return WriteResult.failed(e);
    }
    if (storeScope.isCurrent) store.adjustCommentCount(postId, -1);
    if (!scope.isCurrent) return const WriteResult.discarded();
    return const WriteResult.applied();
  }

  /// 切換讚：本地先改，失敗只還原這一則；同一則留言的請求還沒回來時忽略再次點擊。
  Future<WriteResult> toggleLike(String commentId) async {
    final current = state.tree.byId[commentId];
    if (current == null || !_likesInFlight.add(commentId)) {
      return const WriteResult.discarded();
    }
    final scope = beginScope();
    final wasLiked = current.viewerLiked;
    state = state.copyWith(
      tree: state.tree.replaced(_withLiked(current, !wasLiked)),
    );
    try {
      await scope.run(_repo.setCommentLiked(commentId, liked: !wasLiked));
      return const WriteResult.applied();
    } on StaleScopeException {
      return const WriteResult.discarded();
    } catch (e) {
      final now = state.tree.byId[commentId];
      if (now != null && now.viewerLiked == !wasLiked) {
        state = state.copyWith(
          tree: state.tree.replaced(_withLiked(now, wasLiked)),
        );
      }
      return WriteResult.failed(e);
    } finally {
      if (scope.isCurrent) _likesInFlight.remove(commentId);
    }
  }

  pb.Comment _withLiked(pb.Comment comment, bool liked) {
    final count = comment.likeCount.toInt();
    final next = liked ? count + 1 : (count > 0 ? count - 1 : 0);
    return comment.deepCopy()
      ..viewerLiked = liked
      ..likeCount = comment.likeCount + (next - count);
  }
}
