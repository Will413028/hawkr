import 'dart:async';

import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/exceptions/app_exception.dart';
import '../../../core/session/viewer_scoped_notifier.dart';
import '../data/social_model.dart';
import 'post_entity_store.dart';

part 'post_entity.g.dart';

/// 找不到不是暫時的失敗，自動重試只會讓畫面在轉圈中多等幾秒才顯示結果。
Duration? _retryUnlessNotFound(int retryCount, Object error) =>
    error is NotFoundException
    ? null
    : ProviderContainer.defaultRetry(retryCount, error);

/// 一則貼文的實體（讚、讚數、留言數、`viewer_*`）。實體存放處已有就直接給，沒有才讀 `GetPost`；
/// 之後跟著存放處更新，所以列表、詳情、預覽卡讀到的是同一份。
@Riverpod(retry: _retryUnlessNotFound)
class PostEntity extends _$PostEntity with ViewerScoped {
  static const _notFound = NotFoundException('找不到這則貼文');

  @override
  FutureOr<Post> build(int postId) {
    bindViewer();
    ref.onDispose(ref.read(postEntityStoreProvider.notifier).retain(postId));
    ref.listen(postEntityStoreProvider.select((s) => s.posts[postId]), (
      _,
      next,
    ) {
      if (next != null && !state.isLoading) state = AsyncData(next);
    });
    ref.listen(postEntityStoreProvider.select((s) => s.gone.contains(postId)), (
      _,
      gone,
    ) {
      if (gone) state = AsyncError(_notFound, StackTrace.current);
    });
    final cached = ref.read(postEntityStoreProvider).posts[postId];
    if (cached != null) return cached;
    return _load();
  }

  Future<Post> _load() async {
    final scope = beginScope();
    return scope.run(ref.read(postEntityStoreProvider.notifier).fetch(postId));
  }
}

/// 列表項目與預覽卡讀的快照：只看實體存放處已有的貼文，不發請求。
/// 觀看者換了、存放處清空時是 null，畫面據此收掉；需要補讀的詳情用 [postEntityProvider]。
@riverpod
Post? postSnapshot(Ref ref, int postId) {
  ref.onDispose(ref.read(postEntityStoreProvider.notifier).retain(postId));
  return ref.watch(postEntityStoreProvider.select((s) => s.posts[postId]));
}

/// 列表持有的 id 一個快照都沒有：觀看者換了、存放處清空，列表重建又失敗，留下的是舊 id。
/// 畫面據此顯示錯誤與重試，不渲染空白頁。
bool watchListUnavailable(WidgetRef ref, List<int> ids) => ref.watch(
  postEntityStoreProvider.select(
    (s) => ids.isNotEmpty && ids.every((id) => !s.posts.containsKey(id)),
  ),
);
