import 'dart:async';

import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:uuid/uuid.dart';

import '../../../core/session/viewer_scoped_notifier.dart';
import '../data/social_content_repository.dart';
import 'post_entity_store.dart';
import 'thread_tree.dart';
import 'write_result.dart';

part 'comments_notifier.g.dart';

enum CommentsPhase {
  /// 第一次讀取中（或失敗後重試中）。
  loading,
  ready,

  /// 第一次讀取失敗，沒有任何可顯示的留言。
  failed,
}

class CommentsState {
  CommentsState({
    this.phase = CommentsPhase.loading,
    ThreadTree<pb.Comment>? tree,
    this.nextPageToken = '',
    this.loadingReplies = const <String>{},
    this.sending = false,
  }) : tree = tree ?? ThreadTree<pb.Comment>(commentCodec);

  final CommentsPhase phase;
  final ThreadTree<pb.Comment> tree;
  final String nextPageToken;
  final Set<String> loadingReplies;
  final bool sending;

  CommentsState copyWith({
    CommentsPhase? phase,
    ThreadTree<pb.Comment>? tree,
    String? nextPageToken,
    Set<String>? loadingReplies,
    bool? sending,
  }) => CommentsState(
    phase: phase ?? this.phase,
    tree: tree ?? this.tree,
    nextPageToken: nextPageToken ?? this.nextPageToken,
    loadingReplies: loadingReplies ?? this.loadingReplies,
    sending: sending ?? this.sending,
  );
}

/// 一則貼文的留言：留言以 id 存一份，回覆以 parentId 分組，分頁、回覆展開與寫入都在這裡。
///
/// 換觀看者時整份重建清空；每個跨 await 的路徑回來先比對世代，過期就丟棄。重新整理另有一個序號，
/// 讓整理之前發出的分頁與展開不會蓋掉新讀到的第一頁。重讀失敗保留舊值；留言數的變動寫回貼文實體。
@riverpod
class Comments extends _$Comments with ViewerScoped {
  static const _uuid = Uuid();

  int _epoch = 0;
  bool _loadingMore = false;
  final Set<String> _likesInFlight = <String>{};

  @override
  CommentsState build(int postId) {
    bindViewer();
    _epoch++;
    _loadingMore = false;
    _likesInFlight.clear();
    final scope = beginScope();
    scheduleMicrotask(() {
      if (scope.isCurrent) unawaited(_load(scope));
    });
    return CommentsState();
  }

  SocialContentRepository get _repo =>
      ref.read(socialContentRepositoryProvider);

  /// 讀第一頁並取代現有內容。回傳是否成功；失敗時保留現有留言（從沒讀成功過則進入 [CommentsPhase.failed]）。
  /// 使用者要求的重讀（下拉、重試）：留言與貼文實體（留言權限、身分、留言數）一起重讀。
  Future<bool> reload() {
    unawaited(ref.read(postEntityStoreProvider.notifier).refresh(postId));
    return _load(beginScope());
  }

  Future<bool> _load(ViewerScope scope) async {
    final epoch = ++_epoch;
    if (state.phase == CommentsPhase.failed) {
      state = state.copyWith(phase: CommentsPhase.loading);
    }
    final pb.ListCommentsResponse response;
    try {
      response = await scope.run(_repo.listComments(postId.toString()));
    } on StaleScopeException {
      return true;
    } catch (_) {
      if (epoch != _epoch) return true;
      if (state.phase != CommentsPhase.ready) {
        state = state.copyWith(phase: CommentsPhase.failed);
      }
      return false;
    }
    if (epoch != _epoch) return true;
    _loadingMore = false;
    state = CommentsState(
      phase: CommentsPhase.ready,
      tree: ThreadTree<pb.Comment>(commentCodec).withRoots(response.comments),
      nextPageToken: response.nextPageToken,
      sending: state.sending,
    );
    return true;
  }

  Future<bool> loadMore() async {
    final token = state.nextPageToken;
    if (token.isEmpty || _loadingMore) return true;
    final scope = beginScope();
    final epoch = _epoch;
    _loadingMore = true;
    try {
      final response = await scope.run(
        _repo.listComments(postId.toString(), pageToken: token),
      );
      if (epoch != _epoch) return true;
      state = state.copyWith(
        tree: state.tree.appendRoots(response.comments),
        nextPageToken: response.nextPageToken,
      );
      return true;
    } on StaleScopeException {
      return true;
    } catch (_) {
      return epoch != _epoch;
    } finally {
      if (scope.isCurrent && epoch == _epoch) _loadingMore = false;
    }
  }

  /// 展開一則留言的全部回覆。
  Future<bool> loadReplies(String parentId) async {
    if (state.loadingReplies.contains(parentId)) return true;
    final scope = beginScope();
    final epoch = _epoch;
    state = state.copyWith(loadingReplies: {...state.loadingReplies, parentId});
    try {
      final all = <pb.Comment>[];
      var token = '';
      do {
        final response = await scope.run(
          _repo.listComments(
            postId.toString(),
            parentId: parentId,
            pageToken: token,
          ),
        );
        all.addAll(response.comments);
        token = response.nextPageToken;
      } while (token.isNotEmpty);
      if (epoch != _epoch) return true;
      state = state.copyWith(tree: state.tree.setReplies(parentId, all));
      return true;
    } on StaleScopeException {
      return true;
    } catch (_) {
      return epoch != _epoch;
    } finally {
      if (scope.isCurrent && epoch == _epoch) {
        state = state.copyWith(
          loadingReplies: {...state.loadingReplies}..remove(parentId),
        );
      }
    }
  }

  /// 新增留言；[parentId] 給了是回覆。成功後加進列表並把貼文的留言數加一。
  Future<WriteResult> create(
    String body, {
    String? parentId,
    String? replyToId,
  }) async {
    if (state.sending) return const WriteResult.discarded();
    final scope = beginScope();
    state = state.copyWith(sending: true);
    final pb.Comment created;
    try {
      created = await scope.run(
        _repo.createComment(
          postId.toString(),
          body,
          parentId: parentId,
          replyToId: replyToId,
          requestId: _uuid.v4(),
        ),
      );
    } on StaleScopeException {
      return const WriteResult.discarded();
    } catch (e) {
      state = state.copyWith(sending: false);
      return WriteResult.failed(e);
    }
    state = state.copyWith(tree: state.tree.added(created), sending: false);
    ref.read(postEntityStoreProvider.notifier).adjustCommentCount(postId, 1);
    return const WriteResult.applied();
  }

  /// 刪除一則留言：先在本地套用，伺服器失敗只還原這一則；觀看者換了就丟棄。
  Future<WriteResult> delete(pb.Comment comment) async {
    final removal = state.tree.removed(comment.id);
    if (removal == null) return const WriteResult.discarded();
    final scope = beginScope();
    state = state.copyWith(tree: removal.tree);
    try {
      await scope.run(_repo.deleteComment(comment));
    } on StaleScopeException {
      return const WriteResult.discarded();
    } catch (e) {
      state = state.copyWith(tree: removal.undo(state.tree));
      return WriteResult.failed(e);
    }
    ref.read(postEntityStoreProvider.notifier).adjustCommentCount(postId, -1);
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
