import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../social/presentation/post_entity_store.dart';
import 'block_target.dart';
import 'blocking_repository.dart';

part 'social_blocker.g.dart';

@Riverpod(keepAlive: true)
SocialBlocker socialBlocker(Ref ref) => SocialBlocker(ref);

/// 社群封鎖的唯一入口：送出封鎖，成功後在同一處移除被封鎖店家已載入的貼文。
/// keepAlive，畫面在等待期間被移除也不會讓後續步驟用到已釋放的 `ref`。
class SocialBlocker {
  SocialBlocker(this._ref);

  final Ref _ref;

  Future<void> block(BlockTarget target) async {
    await _ref.read(blockingRepositoryProvider).block(target);
    final storeId = target.blockedStoreId;
    if (storeId == null) return;
    _ref
        .read(postEntityStoreProvider.notifier)
        .removeStorePosts(storeId, alsoPostId: target.originPostId);
  }
}
