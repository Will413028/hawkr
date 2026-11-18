import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/exceptions/app_exception.dart';
import '../../../core/session/viewer_scoped_notifier.dart';
import '../data/social_content_repository.dart';
import '../data/social_model.dart';

part 'post_entity_store.g.dart';

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
@Riverpod(keepAlive: true)
class PostEntityStore extends _$PostEntityStore with ViewerScoped {
  final Set<int> _liking = <int>{};

  @override
  PostEntities build() {
    bindViewer();
    _liking.clear();
    return const PostEntities();
  }

  /// 列表或詳情讀到的貼文寫入實體；已被標為不可見的 id 重新可見。
  void seed(Iterable<Post> posts) {
    final next = Map<int, Post>.of(state.posts);
    var gone = state.gone;
    for (final post in posts) {
      next[post.id] = post;
      if (gone.contains(post.id)) gone = {...gone}..remove(post.id);
    }
    state = PostEntities(posts: next, gone: gone);
  }

  void remove(int postId) {
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
    seed([post]);
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
