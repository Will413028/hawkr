import 'dart:async';

import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:uuid/uuid.dart';

import '../../../core/session/viewer_scoped_notifier.dart';
import '../data/social_content_repository.dart';
import 'comments_notifier.dart' show CommentsPhase;
import 'thread_tree.dart';
import 'write_result.dart';

part 'forum_notifier.g.dart';

class ForumState {
  ForumState({
    this.phase = CommentsPhase.loading,
    ThreadTree<pb.ForumEntry>? tree,
    this.viewerCanComment = false,
    this.viewerActor,
    this.submitting = false,
  }) : tree = tree ?? ThreadTree<pb.ForumEntry>(forumEntryCodec);

  final CommentsPhase phase;
  final ThreadTree<pb.ForumEntry> tree;

  /// 觀看者能不能新增頂層項目（`ListForumEntries` 回應）；回覆與編輯另看項目的 `viewer_can_*`。
  final bool viewerCanComment;

  /// 觀看者在這家店討論區發言時的身分；訪客 null。
  final pb.Actor? viewerActor;
  final bool submitting;

  ForumState copyWith({
    CommentsPhase? phase,
    ThreadTree<pb.ForumEntry>? tree,
    bool? viewerCanComment,
    pb.Actor? viewerActor,
    bool clearViewerActor = false,
    bool? submitting,
  }) => ForumState(
    phase: phase ?? this.phase,
    tree: tree ?? this.tree,
    viewerCanComment: viewerCanComment ?? this.viewerCanComment,
    viewerActor: clearViewerActor ? null : (viewerActor ?? this.viewerActor),
    submitting: submitting ?? this.submitting,
  );
}

/// 一家店的討論區：項目以 id 存一份，回覆以 parentId 分組，讀取、回覆展開與寫入都在這裡（沒有按讚）。
///
/// 世代與序號規則同 `Comments`：換觀看者整份重建清空，每個 await 回來先比對；重讀失敗保留舊值。
@riverpod
class Forum extends _$Forum with ViewerScoped {
  static const _uuid = Uuid();

  int _epoch = 0;

  @override
  ForumState build(String storeId) {
    bindViewer();
    _epoch++;
    final scope = beginScope();
    scheduleMicrotask(() {
      if (scope.isCurrent) unawaited(_load(scope));
    });
    return ForumState();
  }

  SocialContentRepository get _repo =>
      ref.read(socialContentRepositoryProvider);

  Future<bool> reload() => _load(beginScope());

  Future<bool> _load(ViewerScope scope) async {
    final epoch = ++_epoch;
    if (state.phase == CommentsPhase.failed) {
      state = state.copyWith(phase: CommentsPhase.loading);
    }
    final pb.ListForumEntriesResponse response;
    try {
      response = await scope.run(_repo.listForumEntries(storeId));
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
    state = ForumState(
      phase: CommentsPhase.ready,
      tree: ThreadTree<pb.ForumEntry>(
        forumEntryCodec,
      ).withRoots(response.entries),
      viewerCanComment: response.viewerCanComment,
      viewerActor: response.hasViewerActor() ? response.viewerActor : null,
      submitting: state.submitting,
    );
    for (final entry in response.entries) {
      if (entry.replyCount.toInt() > 0) {
        unawaited(_loadReplies(entry.id, scope, epoch));
      }
    }
    return true;
  }

  Future<void> _loadReplies(
    String parentId,
    ViewerScope scope,
    int epoch,
  ) async {
    try {
      final all = <pb.ForumEntry>[];
      var token = '';
      do {
        final response = await scope.run(
          _repo.listForumEntries(storeId, parentId: parentId, pageToken: token),
        );
        all.addAll(response.entries);
        token = response.nextPageToken;
      } while (token.isNotEmpty);
      if (all.isEmpty || epoch != _epoch) return;
      state = state.copyWith(tree: state.tree.setReplies(parentId, all));
    } catch (_) {
      return;
    }
  }

  /// 新增項目；[parentId] 給了是回覆。成功後加進列表。
  Future<WriteResult> create(
    String body, {
    String? parentId,
    String? replyToId,
  }) async {
    if (state.submitting) return const WriteResult.discarded();
    final scope = beginScope();
    state = state.copyWith(submitting: true);
    final pb.ForumEntry created;
    try {
      created = await scope.run(
        _repo.createForumEntry(
          storeId,
          body,
          parentId: parentId,
          replyToId: replyToId,
          requestId: _uuid.v4(),
        ),
      );
    } on StaleScopeException {
      return const WriteResult.discarded();
    } catch (e) {
      state = state.copyWith(submitting: false);
      return WriteResult.failed(e);
    }
    state = state.copyWith(tree: state.tree.added(created), submitting: false);
    return const WriteResult.applied();
  }

  /// 編輯內文：本地先改，失敗只還原這一則；成功後重讀，拿到新的 etag。
  Future<WriteResult> edit(pb.ForumEntry entry, String body) async {
    if (state.submitting) return const WriteResult.discarded();
    final current = state.tree.byId[entry.id];
    if (current == null) return const WriteResult.discarded();
    final scope = beginScope();
    state = state.copyWith(
      tree: state.tree.replaced(current.deepCopy()..body = body),
      submitting: true,
    );
    try {
      await scope.run(_repo.updateForumEntry(entry, body));
    } on StaleScopeException {
      return const WriteResult.discarded();
    } catch (e) {
      final now = state.tree.byId[entry.id];
      state = state.copyWith(
        tree: now != null && now.body == body
            ? state.tree.replaced(now.deepCopy()..body = current.body)
            : state.tree,
        submitting: false,
      );
      return WriteResult.failed(e);
    }
    state = state.copyWith(submitting: false);
    await _load(scope);
    return const WriteResult.applied();
  }

  /// 刪除一則：先在本地套用，失敗只還原這一則；觀看者換了就丟棄。
  Future<WriteResult> delete(pb.ForumEntry entry) async {
    final removal = state.tree.removed(entry.id);
    if (removal == null) return const WriteResult.discarded();
    final scope = beginScope();
    state = state.copyWith(tree: removal.tree);
    try {
      await scope.run(_repo.deleteForumEntry(entry));
    } on StaleScopeException {
      return const WriteResult.discarded();
    } catch (e) {
      state = state.copyWith(tree: removal.undo(state.tree));
      return WriteResult.failed(e);
    }
    return const WriteResult.applied();
  }
}
