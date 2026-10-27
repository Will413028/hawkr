import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../data/social_model.dart';
import '../data/social_repository.dart';

part 'post_feed_controller.g.dart';

/// 貼文 feed controller，支援 TikTok-style 無限往下滑。
///
/// 規則：
/// - 初始 build → 拿 page 1（20 筆），cache 在 state
/// - [loadMore] → 拿 page+1，dedup by id 後 append；< pageSize 視為已到底
/// - [likePost] → optimistic update 直接改 state，不 invalidateSelf 避免重置已載入頁
@riverpod
class PostFeedController extends _$PostFeedController {
  static const _pageSize = 20;
  int _currentPage = 1;
  bool _hasMore = true;
  bool _loadingMore = false;

  @override
  Future<List<Post>> build({int? merchantId}) async {
    ref.keepAlive();
    _currentPage = 1;
    _hasMore = true;
    _loadingMore = false;
    final posts = await ref
        .watch(socialRepositoryProvider)
        .getPosts(merchantId: merchantId, page: 1, pageSize: _pageSize);
    if (posts.length < _pageSize) _hasMore = false;
    return posts;
  }

  /// 預先載入下一頁。用於 PageView onPageChanged 距底部 N 篇時呼叫。
  /// 內部 debounce 重複呼叫；後端回傳 < pageSize 視為已到底。
  Future<void> loadMore() async {
    if (_loadingMore || !_hasMore) return;
    final current = state.value;
    if (current == null) return; // 初始尚未載入
    _loadingMore = true;
    try {
      final next = _currentPage + 1;
      final newPosts = await ref
          .read(socialRepositoryProvider)
          .getPosts(merchantId: merchantId, page: next, pageSize: _pageSize);
      if (newPosts.length < _pageSize) _hasMore = false;
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
      _currentPage = next;
    } catch (_) {
      // 失敗就先不處理；保留 _currentPage 不變，下次仍可重試
    } finally {
      _loadingMore = false;
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
      await ref.read(socialRepositoryProvider).likePost(postId);
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
