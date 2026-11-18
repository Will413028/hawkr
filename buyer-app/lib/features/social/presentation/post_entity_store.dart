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
/// 換觀看者時整份重建清空；讚是集中的 mutation（樂觀、以 id 回滾、世代檢查）。
/// 實體只在被引用時留著：列表以 [setListed] 登記持有的 id，畫面以 [retain] 監看；
/// 兩者都沒有之後，等 [postEntityGcTimeProvider] 的時間才移除，期間重新被引用就保留。
@Riverpod(keepAlive: true)
class PostEntityStore extends _$PostEntityStore with ViewerScoped {
  final Set<int> _liking = <int>{};
  final Map<int, int> _watchers = <int, int>{};
  final Map<Object, Set<int>> _listed = <Object, Set<int>>{};
  final Map<int, Timer> _evictions = <int, Timer>{};
  final Set<int> _stored = <int>{};
  Duration _gcTime = const Duration(minutes: 5);
  bool _disposed = false;

  @override
  PostEntities build() {
    bindViewer();
    _liking.clear();
    _stored.clear();
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
      _listed.values.any((ids) => ids.contains(postId));

  void _cancelEviction(int postId) => _evictions.remove(postId)?.cancel();

  void _scheduleEvictionIfUnreferenced(int postId) {
    if (_disposed || _isReferenced(postId) || !_stored.contains(postId)) {
      return;
    }
    _evictions[postId]?.cancel();
    final scope = beginScope();
    _evictions[postId] = Timer(_gcTime, () {
      _evictions.remove(postId);
      if (!scope.isCurrent || _isReferenced(postId)) return;
      _stored.remove(postId);
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

  /// 列表或詳情讀到的貼文寫入實體；已被標為不可見的 id 重新可見。
  void seed(Iterable<Post> posts) {
    final next = Map<int, Post>.of(state.posts);
    var gone = state.gone;
    for (final post in posts) {
      next[post.id] = post;
      _stored.add(post.id);
      if (gone.contains(post.id)) gone = {...gone}..remove(post.id);
    }
    state = PostEntities(posts: next, gone: gone);
    for (final post in posts) {
      _scheduleEvictionIfUnreferenced(post.id);
    }
  }

  void remove(int postId) {
    _cancelEviction(postId);
    _stored.remove(postId);
    final next = Map<int, Post>.of(state.posts)..remove(postId);
    state = PostEntities(posts: next, gone: {...state.gone, postId});
  }

  void _put(Post post) {
    state = PostEntities(
      posts: {...state.posts, post.id: post},
      gone: state.gone,
    );
  }

  /// 切換讚；方向取自呼叫當下的實體。失敗以 id 還原並回 false；途中觀看者換了或實體已重讀，結果丟棄並回 false。
  Future<bool> toggleLike(int postId) async {
    final before = state.posts[postId];
    if (before == null || !_liking.add(postId)) return false;
    final scope = beginScope();
    final liked = !before.liked;
    _put(
      before.copyWith(
        liked: liked,
        likeCount: liked
            ? before.likeCount + 1
            : (before.likeCount - 1).clamp(0, 1 << 30),
      ),
    );
    try {
      await scope.run(
        ref
            .read(socialContentRepositoryProvider)
            .setPostLiked(postId, liked: liked),
      );
      return true;
    } on StaleScopeException {
      return false;
    } catch (_) {
      final current = state.posts[postId];
      if (current != null) {
        _put(
          current.copyWith(liked: before.liked, likeCount: before.likeCount),
        );
      }
      return false;
    } finally {
      if (scope.isCurrent) _liking.remove(postId);
    }
  }

  /// 讀單一貼文並寫入實體；失敗（含找不到）照丟，由呼叫端決定怎麼處理。
  Future<Post> fetch(int postId) async {
    final scope = beginScope();
    final post = await scope.run(
      ref.read(socialContentRepositoryProvider).getPost(postId),
    );
    if (_stored.contains(postId) || _isReferenced(postId)) seed([post]);
    return post;
  }

  /// 重讀單一貼文。失敗保留舊值並回 false；只有找不到才當成不可見。
  Future<bool> refresh(int postId) async {
    try {
      await fetch(postId);
      return true;
    } on NotFoundException {
      remove(postId);
      return false;
    } catch (_) {
      return false;
    }
  }
}
