import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/session/viewer_scoped_notifier.dart';
import '../data/social_content_repository.dart';
import '../data/social_model.dart';
import 'post_entity_store.dart';

part 'post_feed_controller.g.dart';

/// 貼文列表：只存貼文 id 與下一頁的 token，貼文本身在 [PostEntityStore]。
class PostListState {
  const PostListState({required this.ids, this.nextPageToken = ''});

  final List<int> ids;
  final String nextPageToken;

  bool get hasMore => nextPageToken.isNotEmpty;
}

/// 貼文列表 controller，支援無限往下滑；[merchantId] 為 null 是全平台。
///
/// - 初始 build → 拿第一頁，貼文寫入實體存放處，列表只留 id
/// - [loadMore] → 用目前的 page token 拿下一頁，dedup 後 append；沒有 token 視為已到底
/// - 重新整理或觀看者換了：build 重跑，之前發出的讀取與分頁回來時被丟棄
@riverpod
class PostFeedController extends _$PostFeedController with ViewerScoped {
  bool _loadingMore = false;

  @override
  Future<PostListState> build({int? merchantId}) async {
    ref.keepAlive();
    _loadingMore = false;
    bindViewer();
    final scope = beginScope();
    final repository = ref.watch(socialContentRepositoryProvider);
    final store = ref.read(postEntityStoreProvider.notifier);
    ref.onDispose(() => store.releaseList(this));
    ref.listen(postEntityStoreProvider.select((s) => s.gone), (_, gone) {
      final current = state.value;
      if (current == null || !current.ids.any(gone.contains)) return;
      _publish(
        PostListState(
          ids: current.ids.where((id) => !gone.contains(id)).toList(),
          nextPageToken: current.nextPageToken,
        ),
      );
    });
    final page = await scope.run(
      repository.listPosts(storeId: merchantId?.toString()),
    );
    final ids = page.posts.map((p) => p.id).toList();
    store.setListed(this, ids);
    store.seed(page.posts);
    return PostListState(ids: ids, nextPageToken: page.nextPageToken);
  }

  void _publish(PostListState next, {Iterable<Post> seed = const <Post>[]}) {
    final store = ref.read(postEntityStoreProvider.notifier);
    store.setListed(this, next.ids);
    store.seed(seed);
    state = AsyncData(next);
  }

  /// 預先載入下一頁。用於 PageView onPageChanged 距底部 N 篇時呼叫。
  Future<void> loadMore() async {
    final current = state.value;
    if (_loadingMore || current == null || !current.hasMore) return;
    _loadingMore = true;
    final scope = beginScope();
    try {
      final page = await scope.run(
        ref
            .read(socialContentRepositoryProvider)
            .listPosts(
              storeId: merchantId?.toString(),
              pageToken: current.nextPageToken,
            ),
      );
      final latest = state.value ?? current;
      final existing = latest.ids.toSet();
      _publish(
        PostListState(
          ids: [
            ...latest.ids,
            ...page.posts
                .map((p) => p.id)
                .where((id) => !existing.contains(id)),
          ],
          nextPageToken: page.nextPageToken,
        ),
        seed: page.posts,
      );
    } catch (_) {
      // 失敗保留 page token，下次仍可重試；過期的結果也走這裡丟棄。
    } finally {
      if (scope.isCurrent) _loadingMore = false;
    }
  }
}
