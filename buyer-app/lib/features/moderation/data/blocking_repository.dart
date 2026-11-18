import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';
import 'block_target.dart';

part 'blocking_repository.g.dart';

@riverpod
BlockingRepository blockingRepository(Ref ref) =>
    BlockingRepository(ref.watch(buyerAppClientProvider));

/// 社群的封鎖與封鎖名單（`BuyerAppService`）。聊天與評價的封鎖仍在 `BlockRepository`（舊 REST）。
class BlockingRepository {
  BlockingRepository(this._buyer);

  /// 名單每頁筆數（API 預設 20，上限 100）。
  static const pageSize = 20;

  final BuyerAppServiceClient _buyer;

  /// 封鎖；已經封鎖時也成功。
  Future<void> block(SocialBlockTarget target) async {
    switch (target) {
      case StoreBlockTarget(:final storeId):
        await _buyer.blockStore(buyer.BlockStoreRequest(storeId: storeId));
      case CommentAuthorBlockTarget(:final commentId):
        await _buyer.blockAuthor(
          buyer.BlockAuthorRequest(commentId: commentId),
        );
      case ForumAuthorBlockTarget(:final entryId):
        await _buyer.blockAuthor(
          buyer.BlockAuthorRequest(forumEntryId: entryId),
        );
    }
  }

  Future<void> unblockStore(String storeId) async {
    await _buyer.unblockStore(buyer.UnblockStoreRequest(storeId: storeId));
  }

  Future<void> unblockBuyer(String buyerId) async {
    await _buyer.unblockBuyer(buyer.UnblockBuyerRequest(buyerId: buyerId));
  }

  /// 封鎖的店家一頁；[pageToken] 空字串是第一頁，回應的 `nextPageToken` 空字串表示沒有下一頁。
  Future<buyer.ListBlockedStoresResponse> listBlockedStores({
    String pageToken = '',
  }) => _buyer.listBlockedStores(
    buyer.ListBlockedStoresRequest(pageSize: pageSize, pageToken: pageToken),
  );

  Future<buyer.ListBlockedBuyersResponse> listBlockedBuyers({
    String pageToken = '',
  }) => _buyer.listBlockedBuyers(
    buyer.ListBlockedBuyersRequest(pageSize: pageSize, pageToken: pageToken),
  );
}
