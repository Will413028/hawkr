import 'dart:async';

import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/exceptions/app_exception.dart';
import '../../../core/session/viewer_scoped_notifier.dart';
import '../data/social_content_repository.dart';
import '../data/social_model.dart';

part 'post_entity_store.g.dart';

/// 沒有任何列表持有、也沒有畫面監看的實體，過這麼久才從存放處移除。
final postEntityGcTimeProvider = Provider<Duration>(
  (ref) => const Duration(minutes: 5),
);

/// 目前觀看者看到的貼文實體，以貼文 id 存一份；[gone] 是已確認不可見（找不到）的 id。
class PostEntities {
  const PostEntities({
    this.posts = const <int, Post>{},
    this.gone = const <int>{},
  });

  final Map<int, Post> posts;
  final Set<int> gone;
}

/// 貼文實體的唯一存放處：列表只存 id，畫面透過 `postSnapshotProvider`（列表、預覽卡）與 `postEntityProvider`（詳情）讀這裡的同一份。
///
/// 換觀看者時整份重建清空。伺服器給的值存成基準；讚是集中的 mutation，進行中的意圖以每個 id 一份的
/// 樂觀疊加層蓋在基準上（[PostEntities.posts] 是疊加後的值）：列表或詳情讀到新資料只更新基準、不抹掉疊加層；
/// 請求成功以基準加意圖為準，失敗只丟掉疊加層，不把舊快照寫回較新的基準。
/// 實體只在被引用時留著：列表以 [setListed] 登記持有的 id，畫面以 [retain] 監看；
/// 兩者都沒有之後，等 [postEntityGcTimeProvider] 的時間才移除，期間重新被引用就保留。
@Riverpod(keepAlive: true)
class PostEntityStore extends _$PostEntityStore with ViewerScoped {
  final Map<int, Post> _base = <int, Post>{};
  final Map<int, bool> _intent = <int, bool>{};
  final Map<int, Future<bool>> _likeRuns = <int, Future<bool>>{};
  final Map<int, int> _watchers = <int, int>{};
  final Map<Object, Set<int>> _listed = <Object, Set<int>>{};
  final Map<int, Timer> _evictions = <int, Timer>{};
  Duration _gcTime = const Duration(minutes: 5);
  bool _disposed = false;

  @override
  PostEntities build() {
    bindViewer();
    _base.clear();
    _intent.clear();
    _likeRuns.clear();
    _disposed = false;
    _gcTime = ref.read(postEntityGcTimeProvider);
    _cancelEvictions();
    ref.onDispose(() {
      _disposed = true;
      _cancelEvictions();
    });
    return const PostEntities();
  }

  void _cancelEvictions() {
    for (final timer in _evictions.values) {
      timer.cancel();
    }
    _evictions.clear();
  }

  bool _isReferenced(int postId) =>
      (_watchers[postId] ?? 0) > 0 ||
      _intent.containsKey(postId) ||
      _listed.values.any((ids) => ids.contains(postId));

  void _cancelEviction(int postId) => _evictions.remove(postId)?.cancel();

  void _scheduleEvictionIfUnreferenced(int postId) {
    if (_disposed || _isReferenced(postId) || !_base.containsKey(postId)) {
      return;
    }
    _evictions[postId]?.cancel();
    final scope = beginScope();
    _evictions[postId] = Timer(_gcTime, () {
      _evictions.remove(postId);
      if (!scope.isCurrent || _isReferenced(postId)) return;
      _base.remove(postId);
      final next = Map<int, Post>.of(state.posts)..remove(postId);
      state = PostEntities(posts: next, gone: state.gone);
    });
  }

  /// 畫面監看一則貼文；回傳的函式在不再監看時呼叫。
  void Function() retain(int postId) {
    _watchers[postId] = (_watchers[postId] ?? 0) + 1;
    _cancelEviction(postId);
    return () {
      final left = (_watchers[postId] ?? 1) - 1;
      if (left <= 0) {
        _watchers.remove(postId);
      } else {
        _watchers[postId] = left;
      }
      _scheduleEvictionIfUnreferenced(postId);
    };
  }

  /// [owner]（一份列表）目前持有的貼文 id；整組取代前一次登記。
  void setListed(Object owner, Iterable<int> ids) {
    final next = ids.toSet();
    final previous = _listed[owner] ?? const <int>{};
    _listed[owner] = next;
    for (final id in next) {
      _cancelEviction(id);
    }
    for (final id in previous.difference(next)) {
      _scheduleEvictionIfUnreferenced(id);
    }
  }

  void releaseList(Object owner) {
    final previous = _listed.remove(owner);
    if (previous == null) return;
    for (final id in previous) {
      _scheduleEvictionIfUnreferenced(id);
    }
  }

  Post _overlaid(Post base, bool? intent) {
    if (intent == null || base.liked == intent) return base;
    return base.copyWith(
      liked: intent,
      likeCount: intent
          ? base.likeCount + 1
          : (base.likeCount - 1).clamp(0, 1 << 30),
    );
  }

  void _publish(int postId) {
    final base = _base[postId];
    if (base == null) return;
    state = PostEntities(
      posts: {...state.posts, postId: _overlaid(base, _intent[postId])},
      gone: state.gone,
    );
  }

  /// 列表或詳情讀到的貼文寫入基準；已被標為不可見的 id 重新可見，進行中的讚疊加層保留。
  /// 呼叫端在列表登記（[setListed]）之後才 seed，不會替即將被持有的 id 排移除計時。
  void seed(Iterable<Post> posts) {
    final next = Map<int, Post>.of(state.posts);
    var gone = state.gone;
    final fresh = <int>[];
    for (final post in posts) {
      _base[post.id] = post;
      next[post.id] = _overlaid(post, _intent[post.id]);
      fresh.add(post.id);
      if (gone.contains(post.id)) gone = {...gone}..remove(post.id);
    }
    state = PostEntities(posts: next, gone: gone);
    for (final id in fresh) {
      _scheduleEvictionIfUnreferenced(id);
    }
  }

  /// 留言數的本地增減（自己新增或刪除留言之後）；下一次讀到伺服器的值就以伺服器為準。
  void adjustCommentCount(int postId, int delta) {
    final base = _base[postId];
    if (base == null) return;
    _base[postId] = base.copyWith(
      commentCount: (base.commentCount + delta).clamp(0, 1 << 30),
    );
    _publish(postId);
  }

  void remove(int postId) {
    _cancelEviction(postId);
    _base.remove(postId);
    _intent.remove(postId);
    final next = Map<int, Post>.of(state.posts)..remove(postId);
    state = PostEntities(posts: next, gone: {...state.gone, postId});
  }

  /// 切換讚；方向取自呼叫當下（含樂觀疊加層）的實體。每次點擊立刻更新疊加層，請求依 id 串行，
  /// 最後一次意圖和伺服器狀態不同就再送一次。回傳最後一輪是否成功；途中觀看者換了回 false。
  Future<bool> toggleLike(int postId) {
    final current = state.posts[postId];
    if (current == null) return Future.value(false);
    _intent[postId] = !current.liked;
    _publish(postId);
    return _likeRuns[postId] ??= _driveLike(postId, beginScope());
  }

  Future<bool> _driveLike(int postId, ViewerScope scope) async {
    try {
      while (true) {
        final sent = _intent[postId];
        if (sent == null) return true;
        try {
          await scope.run(
            ref
                .read(socialContentRepositoryProvider)
                .setPostLiked(postId, liked: sent),
          );
        } on StaleScopeException {
          return false;
        } on NotFoundException {
          remove(postId);
          return false;
        } catch (_) {
          _intent.remove(postId);
          _publish(postId);
          _scheduleEvictionIfUnreferenced(postId);
          return false;
        }
        final base = _base[postId];
        if (base != null) _base[postId] = _overlaid(base, sent);
        if (_intent[postId] == sent) {
          _intent.remove(postId);
          _publish(postId);
          _scheduleEvictionIfUnreferenced(postId);
          return true;
        }
        _publish(postId);
      }
    } finally {
      if (scope.isCurrent) _likeRuns.remove(postId);
    }
  }

  /// 讀單一貼文並寫入實體；失敗照丟，由呼叫端決定怎麼處理。找不到會把 id 標為不可見（列表據此移除）。
  Future<Post> fetch(int postId) async {
    final scope = beginScope();
    final Post post;
    try {
      post = await scope.run(
        ref.read(socialContentRepositoryProvider).getPost(postId),
      );
    } on NotFoundException {
      remove(postId);
      rethrow;
    }
    if (_base.containsKey(postId) || _isReferenced(postId)) seed([post]);
    return post;
  }

  /// 重讀單一貼文。失敗保留舊值；只有找不到才當成不可見。
  Future<PostRefreshResult> refresh(int postId) async {
    try {
      await fetch(postId);
      return PostRefreshResult.refreshed;
    } on NotFoundException {
      return PostRefreshResult.notFound;
    } catch (_) {
      return PostRefreshResult.failed;
    }
  }
}

enum PostRefreshResult { refreshed, notFound, failed }
