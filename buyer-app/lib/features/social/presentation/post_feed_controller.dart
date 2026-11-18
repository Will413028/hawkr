import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../../core/session/session_provider.dart';
import '../data/social_content_repository.dart';
import '../data/social_model.dart';

part 'post_feed_controller.g.dart';

/// 貼文 feed controller，支援 TikTok-style 無限往下滑。
///
/// 規則：
/// - 初始 build → 拿第一頁（20 筆），cache 在 state
/// - [loadMore] → 用上一頁的 page token 拿下一頁，dedup by id 後 append；沒有 token 視為已到底
/// - [likePost] → optimistic update 直接改 state，不 invalidateSelf 避免重置已載入頁
@riverpod
class PostFeedController extends _$PostFeedController {
  String _nextPageToken = '';
  bool _hasMore = true;
  bool _loadingMore = false;

  /// 每次重建加一；舊一輪發出的 [loadMore] 回來時對不上就丟掉，不寫 token 也不接到新列表。
  int _generation = 0;

  @override
  Future<List<Post>> build({int? merchantId}) async {
    ref.keepAlive();
    _nextPageToken = '';
    _hasMore = false;
    _loadingMore = false;
    _generation++;
    ref.listen(sessionProvider, (previous, next) {
      if (previous?.value?.userId != next.value?.userId) ref.invalidateSelf();
    });
    final page = await ref
        .watch(socialContentRepositoryProvider)
        .listPosts(storeId: merchantId?.toString());
    _nextPageToken = page.nextPageToken;
    _hasMore = page.nextPageToken.isNotEmpty;
    return page.posts;
  }

  /// 預先載入下一頁。用於 PageView onPageChanged 距底部 N 篇時呼叫。
  /// 內部 debounce 重複呼叫；回應沒有下一頁的 token 視為已到底。
  Future<void> loadMore() async {
    if (_loadingMore || !_hasMore) return;
    final current = state.value;
    if (current == null) return; // 初始尚未載入
    _loadingMore = true;
    final generation = _generation;
    try {
      final page = await ref
          .read(socialContentRepositoryProvider)
          .listPosts(
            storeId: merchantId?.toString(),
            pageToken: _nextPageToken,
          );
      if (generation != _generation) return;
      final newPosts = page.posts;
      _nextPageToken = page.nextPageToken;
      _hasMore = page.nextPageToken.isNotEmpty;
      if (newPosts.isNotEmpty) {
        // dedup by id（避免後端 page boundary 重複）
        final existingIds = current.map((p) => p.id).toSet();
        final additions = newPosts
            .where((p) => !existingIds.contains(p.id))
            .toList();
        if (additions.isNotEmpty) {
          state = AsyncData([...current, ...additions]);
        }
      }
    } catch (_) {
      // 失敗就先不處理；保留 page token 不變，下次仍可重試
    } finally {
      if (generation == _generation) _loadingMore = false;
    }
  }

  /// Optimistic like：直接改 state（不 invalidateSelf 否則會重置已載入頁），
  /// 後端失敗就 rollback。
  Future<void> likePost(int postId) async {
    final current = state.value;
    if (current == null) return;

    final originalIndex = current.indexWhere((p) => p.id == postId);
    if (originalIndex < 0) return;
    final original = current[originalIndex];

    // 1. 樂觀切換
    final toggled = original.copyWith(
      liked: !original.liked,
      likeCount: original.liked
          ? (original.likeCount - 1).clamp(0, 1 << 30)
          : original.likeCount + 1,
    );
    final optimistic = [...current];
    optimistic[originalIndex] = toggled;
    state = AsyncData(optimistic);

    // 2. 呼叫後端
    try {
      await ref
          .read(socialContentRepositoryProvider)
          .setPostLiked(postId, liked: toggled.liked);
    } catch (_) {
      // 失敗 rollback
      final rollback = state.value;
      if (rollback != null) {
        final idx = rollback.indexWhere((p) => p.id == postId);
        if (idx >= 0) {
          final reverted = [...rollback];
          reverted[idx] = original;
          state = AsyncData(reverted);
        }
      }
      rethrow;
    }
  }
}
