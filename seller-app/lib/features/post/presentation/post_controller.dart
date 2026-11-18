import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart'
    show SellerCapability;
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:flutter_riverpod/flutter_riverpod.dart' show Provider;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../seller/data/current_store.dart';
import '../data/post_repository.dart';
import '../data/post_model.dart';

part 'post_controller.g.dart';

/// 店家有 MARKETING 能力才能建立、修改、發布貼文。能力還沒讀到時不下結論（true），伺服器是最後一道。
final canMarketPostsProvider = Provider<bool>(
  (ref) =>
      ref
          .watch(myStoreSnapshotProvider)
          ?.can(SellerCapability.SELLER_CAPABILITY_MARKETING) ??
      true,
);

/// 使用者面的錯誤提示只處理 `Exception`（網路、`ConnectException`）；`Error`（型別、狀態錯誤）是程式缺陷，原樣往上丟。
void rethrowIfProgrammingError(Object error, StackTrace stackTrace) {
  if (error is Error) Error.throwWithStackTrace(error, stackTrace);
}

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

  /// 向伺服器重讀這一篇並換進列表（排程時間沿用列表上的值）；失敗丟例外，不吞掉。
  /// 貼文不在已載入的列表時只回傳，不改列表。
  Future<Post> refreshPost(int postId) async {
    final latest = await ref.read(postRepositoryProvider).getMyPost(postId);
    final prevState = state;
    if (prevState is AsyncData<List<Post>>) {
      final current = prevState.value;
      final index = current.indexWhere((p) => p.id == postId);
      if (index == -1) return latest;
      final merged = latest.copyWith(
        scheduledPublishAt: current[index].scheduledPublishAt,
      );
      state = AsyncData([
        for (var i = 0; i < current.length; i++)
          i == index ? merged : current[i],
      ]);
      return merged;
    }
    return latest;
  }

  /// etag 與伺服器不符（aborted）代表手上的貼文是舊的：重讀那一篇，讓下一次重試帶新的 etag。
  /// 重讀失敗時丟重讀的錯誤。
  Future<void> _refreshOnConflict(int postId, ConnectException error) async {
    if (error.code == Code.aborted) await refreshPost(postId);
  }

  /// 發布或下架。不做樂觀更新：發布狀態是後端算出來的（排程會被一併結束），
  /// 成功後重讀列表，失敗時狀態不變、錯誤交給呼叫端顯示。
  ///
  /// 發布已成功但重讀失敗時，不丟例外（呼叫端會把例外當成「發布失敗」），也不丟掉現有的
  /// 列表：state 變成帶著舊列表的 [AsyncError]，畫面繼續顯示舊列表並停用發布與下架
  /// （舊列表上的狀態已經不是真的），直到下一次重讀成功。回傳是否重讀成功。
  Future<bool> setPublished(int postId, {required bool publish}) async {
    final repo = ref.read(postRepositoryProvider);
    final etag =
        state.value?.where((p) => p.id == postId).firstOrNull?.etag ??
        (await repo.getMyPost(postId)).etag;
    try {
      if (publish) {
        await repo.publishPostOrThrow(postId, etag: etag);
      } else {
        await repo.unpublishPostOrThrow(postId, etag: etag);
      }
    } on ConnectException catch (error) {
      await _refreshOnConflict(postId, error);
      rethrow;
    }
    try {
      state = AsyncData(await repo.listMyPosts());
      return true;
    } catch (e, st) {
      state = AsyncError<List<Post>>(e, st);
      return false;
    }
  }

  /// 修改貼文；成功回傳更新後的貼文（排程時間沿用列表上的值），失敗丟 `ConnectException`。
  Future<Post> updatePost(
    Post post, {
    required String content,
    String? imageStagedToken,
    String? productId,
    bool updateProduct = false,
    List<String> hashtags = const [],
  }) async {
    final Post updated;
    try {
      updated =
          (await ref
                  .read(postRepositoryProvider)
                  .updatePost(
                    post.id,
                    etag: post.etag,
                    content: content,
                    imageStagedToken: imageStagedToken,
                    productId: productId,
                    updateProduct: updateProduct,
                    hashtags: hashtags,
                  ))
              .copyWith(scheduledPublishAt: post.scheduledPublishAt);
    } on ConnectException catch (error) {
      await _refreshOnConflict(post.id, error);
      rethrow;
    }
    final prevState = state;
    if (prevState is AsyncData<List<Post>>) {
      state = AsyncData(
        prevState.value.map((p) => p.id == post.id ? updated : p).toList(),
      );
    }
    return updated;
  }

  /// 從伺服器重讀這篇貼文的 `comment_count`（留言數的唯一來源），失敗時保留現值。
  Future<int?> refreshCommentCount(int postId) async {
    try {
      final fresh = await ref.read(postRepositoryProvider).getMyPost(postId);
      updateCommentCount(postId, fresh.commentCount);
      return fresh.commentCount;
    } catch (_) {
      return null;
    }
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
