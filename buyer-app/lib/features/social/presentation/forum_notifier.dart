import 'dart:async';

import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:uuid/uuid.dart';

import '../../../core/session/viewer_scoped_notifier.dart';
import '../data/social_content_repository.dart';
import 'thread_notifier.dart';
import 'thread_tree.dart';
import 'write_result.dart';

part 'forum_notifier.g.dart';

class ForumState implements ThreadStateView<ForumState, pb.ForumEntry> {
  ForumState({
    this.phase = ThreadPhase.loading,
    ThreadTree<pb.ForumEntry>? tree,
    this.viewerCanComment = false,
    this.viewerActor,
    this.submitting = false,
  }) : tree = tree ?? ThreadTree<pb.ForumEntry>(forumEntryCodec);

  @override
  final ThreadPhase phase;
  @override
  final ThreadTree<pb.ForumEntry> tree;

  /// 觀看者能不能新增頂層項目（`ListForumEntries` 回應）；回覆與編輯另看項目的 `viewer_can_*`。
  final bool viewerCanComment;

  /// 觀看者在這家店討論區發言時的身分；訪客 null。
  final pb.Actor? viewerActor;
  final bool submitting;

  ForumState copyWith({
    ThreadPhase? phase,
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

  @override
  ForumState withThread({
    ThreadPhase? phase,
    ThreadTree<pb.ForumEntry>? tree,
  }) => copyWith(phase: phase, tree: tree);
}

/// 一家店的討論區：項目以 id 存一份，回覆以 parentId 分組；讀取與回覆展開在 [ThreadNotifier]，寫入在這裡（沒有按讚）。
@riverpod
class Forum extends _$Forum
    with ViewerScoped, ThreadNotifier<ForumState, pb.ForumEntry> {
  static const _uuid = Uuid();

  @override
  ForumState build(String storeId) {
    bindViewer();
    startThread();
    return ForumState();
  }

  SocialContentRepository get _repo =>
      ref.read(socialContentRepositoryProvider);

  @override
  bool get autoLoadReplies => true;

  @override
  Future<ThreadPage<pb.ForumEntry>> fetchPage({
    String? parentId,
    String pageToken = '',
  }) async {
    final response = await _repo.listForumEntries(
      storeId,
      parentId: parentId,
      pageToken: pageToken,
    );
    return ThreadPage(response.entries, response.nextPageToken, meta: response);
  }

  @override
  ForumState firstPageState(
    ForumState current,
    ThreadTree<pb.ForumEntry> tree,
    ThreadPage<pb.ForumEntry> page,
  ) {
    final response = page.meta! as pb.ListForumEntriesResponse;
    return ForumState(
      phase: ThreadPhase.ready,
      tree: tree,
      viewerCanComment: response.viewerCanComment,
      viewerActor: response.hasViewerActor() ? response.viewerActor : null,
      submitting: current.submitting,
    );
  }

  Future<bool> reload() => loadFirstPage(beginScope());

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
    applyCreated(created);
    state = state.copyWith(submitting: false);
    return const WriteResult.applied();
  }

  /// 編輯內文：本地先改，失敗只還原這一則；成功以回應裡的完整項目（含新的 etag）取代。
  Future<WriteResult> edit(pb.ForumEntry entry, String body) async {
    if (state.submitting) return const WriteResult.discarded();
    final current = state.tree.byId[entry.id];
    if (current == null) return const WriteResult.discarded();
    final scope = beginScope();
    state = state.copyWith(
      tree: state.tree.replaced(current.deepCopy()..body = body),
      submitting: true,
    );
    final pb.ForumEntry updated;
    try {
      updated = await scope.run(_repo.updateForumEntry(entry, body));
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
    applyReplaced(updated);
    state = state.copyWith(submitting: false);
    return const WriteResult.applied();
  }

  /// 刪除一則：先在本地套用，失敗只還原這一則；觀看者換了就丟棄。
  Future<WriteResult> delete(pb.ForumEntry entry) async {
    final deletion = applyDelete(entry.id);
    if (deletion == null) return const WriteResult.discarded();
    final scope = beginScope();
    try {
      await scope.run(_repo.deleteForumEntry(entry));
    } on StaleScopeException {
      return const WriteResult.discarded();
    } catch (e) {
      rollbackDelete(deletion);
      return WriteResult.failed(e);
    }
    return const WriteResult.applied();
  }
}
