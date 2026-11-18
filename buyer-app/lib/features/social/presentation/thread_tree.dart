import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;

/// [ThreadTree] 需要知道的項目形狀；留言與討論區項目的欄位相同，各給一份。
class ThreadCodec<T> {
  const ThreadCodec({
    required this.id,
    required this.parentId,
    required this.replyCount,
    required this.withReplyCount,
    required this.asPlaceholder,
    required this.isVisible,
  });

  final String Function(T item) id;
  final String? Function(T item) parentId;
  final int Function(T item) replyCount;
  final T Function(T item, int replyCount) withReplyCount;
  final T Function(T item) asPlaceholder;
  final bool Function(T item) isVisible;
}

final ThreadCodec<pb.Comment> commentCodec = ThreadCodec<pb.Comment>(
  id: (c) => c.id,
  parentId: (c) => c.hasParentId() ? c.parentId : null,
  replyCount: (c) => c.replyCount.toInt(),
  withReplyCount: (c, n) =>
      c.deepCopy()..replyCount = c.replyCount + (n - c.replyCount.toInt()),
  asPlaceholder: (c) => c.deepCopy()
    ..state = pb.CommentState.COMMENT_STATE_DELETED
    ..clearBody()
    ..clearAuthor()
    ..viewerCanEdit = false
    ..viewerCanDelete = false
    ..viewerCanReply = false,
  isVisible: (c) => c.state == pb.CommentState.COMMENT_STATE_VISIBLE,
);

final ThreadCodec<pb.ForumEntry> forumEntryCodec = ThreadCodec<pb.ForumEntry>(
  id: (e) => e.id,
  parentId: (e) => e.hasParentId() ? e.parentId : null,
  replyCount: (e) => e.replyCount.toInt(),
  withReplyCount: (e, n) =>
      e.deepCopy()..replyCount = e.replyCount + (n - e.replyCount.toInt()),
  asPlaceholder: (e) => e.deepCopy()
    ..state = pb.CommentState.COMMENT_STATE_DELETED
    ..clearBody()
    ..clearAuthor()
    ..viewerCanEdit = false
    ..viewerCanDelete = false
    ..viewerCanReply = false,
  isVisible: (e) => e.state == pb.CommentState.COMMENT_STATE_VISIBLE,
);

/// 兩層的討論串：項目以 id 存一份，頂層只存 id 的順序，回覆以 parentId 分組。所有操作回傳新的樹。
class ThreadTree<T> {
  const ThreadTree(
    this.codec, {
    this.byId = const {},
    this.rootIds = const [],
    this.replyIds = const {},
  });

  final ThreadCodec<T> codec;
  final Map<String, T> byId;
  final List<String> rootIds;
  final Map<String, List<String>> replyIds;

  List<T> get roots => [for (final id in rootIds) byId[id] as T];

  List<T> repliesOf(String parentId) => [
    for (final id in replyIds[parentId] ?? const <String>[]) byId[id] as T,
  ];

  /// 已載入的頂層與回覆總數。
  int get loadedCount =>
      rootIds.length +
      replyIds.values.fold<int>(0, (sum, ids) => sum + ids.length);

  ThreadTree<T> _copy({
    Map<String, T>? byId,
    List<String>? rootIds,
    Map<String, List<String>>? replyIds,
  }) => ThreadTree<T>(
    codec,
    byId: byId ?? this.byId,
    rootIds: rootIds ?? this.rootIds,
    replyIds: replyIds ?? this.replyIds,
  );

  /// 整組換成新讀到的第一頁；已展開的回覆一併清掉。
  ThreadTree<T> withRoots(Iterable<T> items) =>
      ThreadTree<T>(codec).appendRoots(items);

  ThreadTree<T> appendRoots(Iterable<T> items) {
    final nextById = Map<String, T>.of(byId);
    final nextRoots = List<String>.of(rootIds);
    for (final item in items) {
      final id = codec.id(item);
      if (!nextById.containsKey(id)) nextRoots.add(id);
      nextById[id] = item;
    }
    return _copy(byId: nextById, rootIds: nextRoots);
  }

  ThreadTree<T> setReplies(String parentId, Iterable<T> items) {
    final nextById = Map<String, T>.of(byId);
    final ids = <String>[];
    for (final item in items) {
      final id = codec.id(item);
      nextById[id] = item;
      if (!ids.contains(id)) ids.add(id);
    }
    return _copy(byId: nextById, replyIds: {...replyIds, parentId: ids});
  }

  /// 新留言：頂層接在最後；回覆接在父項目底下，父項目的回覆數加一。
  ThreadTree<T> added(T item) {
    final id = codec.id(item);
    final parentId = codec.parentId(item);
    if (parentId == null) return appendRoots([item]);
    final parent = byId[parentId];
    final nextById = Map<String, T>.of(byId)..[id] = item;
    final ids = List<String>.of(replyIds[parentId] ?? const <String>[]);
    final isNew = !ids.contains(id);
    if (isNew) ids.add(id);
    if (parent != null && isNew) {
      nextById[parentId] = codec.withReplyCount(
        parent,
        codec.replyCount(parent) + 1,
      );
    }
    return _copy(byId: nextById, replyIds: {...replyIds, parentId: ids});
  }

  ThreadTree<T> replaced(T item) {
    final id = codec.id(item);
    if (!byId.containsKey(id)) return this;
    return _copy(byId: {...byId, id: item});
  }

  /// 刪除一則：有回覆的頂層留言變成已刪除的佔位，其餘直接移除；回覆被刪時父項目的回覆數減一，
  /// 已不可見的父項目底下沒有回覆了就一起移除。回傳新樹與只還原這幾個 id 的 undo；找不到 [id] 回 null。
  ({ThreadTree<T> tree, ThreadTree<T> Function(ThreadTree<T> current) undo})?
  removed(String id) {
    final item = byId[id];
    if (item == null) return null;
    final parentId = codec.parentId(item);
    if (parentId == null) {
      final hasReplies =
          codec.replyCount(item) > 0 || (replyIds[id]?.isNotEmpty ?? false);
      if (hasReplies) {
        return (
          tree: _copy(byId: {...byId, id: codec.asPlaceholder(item)}),
          undo: (current) => current.replaced(item),
        );
      }
      final index = rootIds.indexOf(id);
      final ownReplies = replyIds[id];
      final nextById = Map<String, T>.of(byId)..remove(id);
      final nextReplies = Map<String, List<String>>.of(replyIds)..remove(id);
      return (
        tree: _copy(
          byId: nextById,
          rootIds: [
            for (final r in rootIds)
              if (r != id) r,
          ],
          replyIds: nextReplies,
        ),
        undo: (current) => current._reinsertRoot(item, index, ownReplies),
      );
    }
    final siblings = replyIds[parentId] ?? const <String>[];
    final replyIndex = siblings.indexOf(id);
    final remaining = [
      for (final s in siblings)
        if (s != id) s,
    ];
    final parent = byId[parentId];
    final nextById = Map<String, T>.of(byId)..remove(id);
    var nextRoots = rootIds;
    final parentIndex = rootIds.indexOf(parentId);
    var removeParent = false;
    if (parent != null) {
      final count = codec.replyCount(parent);
      if (!codec.isVisible(parent) && remaining.isEmpty && count <= 1) {
        removeParent = true;
        nextById.remove(parentId);
        nextRoots = [
          for (final r in rootIds)
            if (r != parentId) r,
        ];
      } else if (count > 0) {
        nextById[parentId] = codec.withReplyCount(parent, count - 1);
      }
    }
    final nextReplies = Map<String, List<String>>.of(replyIds);
    if (removeParent) {
      nextReplies.remove(parentId);
    } else {
      nextReplies[parentId] = remaining;
    }
    return (
      tree: _copy(byId: nextById, rootIds: nextRoots, replyIds: nextReplies),
      undo: (current) => current._reinsertReply(
        item,
        parentId,
        replyIndex,
        parent: parent,
        parentRootIndex: removeParent ? parentIndex : null,
      ),
    );
  }

  ThreadTree<T> _reinsertRoot(T item, int index, List<String>? ownReplies) {
    final id = codec.id(item);
    final nextRoots = List<String>.of(rootIds);
    if (!nextRoots.contains(id)) {
      nextRoots.insert(index.clamp(0, nextRoots.length), id);
    }
    return _copy(
      byId: {...byId, id: item},
      rootIds: nextRoots,
      replyIds: ownReplies == null
          ? replyIds
          : {...replyIds, id: List<String>.of(ownReplies)},
    );
  }

  ThreadTree<T> _reinsertReply(
    T item,
    String parentId,
    int index, {
    required T? parent,
    required int? parentRootIndex,
  }) {
    final id = codec.id(item);
    final nextById = Map<String, T>.of(byId)..[id] = item;
    var nextRoots = rootIds;
    if (parent != null) {
      if (parentRootIndex != null && !rootIds.contains(parentId)) {
        nextRoots = List<String>.of(rootIds)
          ..insert(parentRootIndex.clamp(0, rootIds.length), parentId);
        nextById[parentId] = parent;
      } else if (byId.containsKey(parentId) && codec.replyCount(parent) > 0) {
        nextById[parentId] = codec.withReplyCount(
          byId[parentId] as T,
          codec.replyCount(byId[parentId] as T) + 1,
        );
      }
    }
    final ids = List<String>.of(replyIds[parentId] ?? const <String>[]);
    if (!ids.contains(id)) ids.insert(index.clamp(0, ids.length), id);
    return _copy(
      byId: nextById,
      rootIds: nextRoots,
      replyIds: {...replyIds, parentId: ids},
    );
  }
}
