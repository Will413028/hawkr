import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../data/post_repository.dart';
import '../data/post_model.dart';

part 'post_controller.g.dart';

@riverpod
class PostList extends _$PostList {
  final Set<int> _pendingDeletes = <int>{};

  @override
  Future<List<Post>> build() async {
    return ref.read(postRepositoryProvider).listMyPosts();
  }

  Future<void> refresh() async {
    state = const AsyncLoading();
    try {
      final posts = await ref.read(postRepositoryProvider).listMyPosts();
      state = AsyncData(posts);
    } catch (e, st) {
      state = AsyncError(e, st);
    }
  }

  /// 靜默刷新：不顯示 loading，直接以最新資料替換 state（避免編輯後圖片消失）
  Future<void> silentRefresh() async {
    try {
      final posts = await ref.read(postRepositoryProvider).listMyPosts();
      state = AsyncData(posts);
    } catch (_) {}
  }

  Future<void> deletePost(int postId) async {
    if (!_pendingDeletes.add(postId)) return;

    // Optimistic update
    final prevState = state;
    if (prevState is AsyncData<List<Post>>) {
      final newList = prevState.value.where((p) => p.id != postId).toList();
      state = AsyncData(newList);
    }

    try {
      await ref.read(postRepositoryProvider).deletePost(postId);
    } catch (error, stackTrace) {
      // Revert on error
      state = prevState;
      Error.throwWithStackTrace(error, stackTrace);
    } finally {
      _pendingDeletes.remove(postId);
    }
  }

  /// 發布或下架。不做樂觀更新：發布狀態是後端算出來的（排程會被一併結束），
  /// 成功後重讀列表，失敗時狀態不變、錯誤交給呼叫端顯示。
  ///
  /// 發布已成功但重讀失敗時，不丟例外（呼叫端會把例外當成「發布失敗」），也不丟掉現有的
  /// 列表：state 變成帶著舊列表的 [AsyncError]，畫面繼續顯示舊列表並停用發布與下架
  /// （舊列表上的狀態已經不是真的），直到下一次重讀成功。回傳是否重讀成功。
  Future<bool> setPublished(int postId, {required bool publish}) async {
    final repo = ref.read(postRepositoryProvider);
    if (publish) {
      await repo.publishPostOrThrow(postId);
    } else {
      await repo.unpublishPostOrThrow(postId);
    }
    try {
      state = AsyncData(await repo.listMyPosts());
      return true;
    } catch (e, st) {
      state = AsyncError<List<Post>>(e, st);
      return false;
    }
  }

  Future<Post?> updatePost(
    int postId, {
    required String content,
    String keepAssetId = '',
    String newStagedToken = '',
    int? productId,
    List<String>? tags,
  }) async {
    final updated = await ref
        .read(postRepositoryProvider)
        .updatePost(
          postId,
          content: content,
          keepAssetId: keepAssetId,
          newStagedToken: newStagedToken,
          productId: productId,
          tags: tags,
        );
    if (updated != null) {
      final prevState = state;
      if (prevState is AsyncData<List<Post>>) {
        final newList = prevState.value
            .map((p) => p.id == postId ? updated : p)
            .toList();
        state = AsyncData(newList);
      }
    }
    return updated;
  }

  void updateCommentCount(int postId, int newCount) {
    final prevState = state;
    if (prevState is AsyncData<List<Post>>) {
      final currentList = prevState.value;
      final index = currentList.indexWhere((p) => p.id == postId);
      if (index != -1) {
        final post = currentList[index];
        final newList = List<Post>.from(currentList);
        newList[index] = post.copyWith(commentCount: newCount);
        state = AsyncData(newList);
      }
    }
  }
}
