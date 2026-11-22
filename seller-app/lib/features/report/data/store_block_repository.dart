import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart' as pb;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';
import '../../../shared/models/buyer_card_info.dart';

part 'store_block_repository.g.dart';

@riverpod
StoreBlockRepository storeBlockRepository(Ref ref) =>
    StoreBlockRepository(ref.watch(sellerClientProvider));

/// 店家封鎖的買家（`BlockedBuyer`）；已刪除的買家只有 [buyerId]。
class BlockedBuyerItem {
  const BlockedBuyerItem({
    required this.buyerId,
    required this.deleted,
    this.name = '',
    this.avatarUrl,
    this.createTime,
  });

  factory BlockedBuyerItem.fromProto(pb.BlockedBuyer b) {
    final buyer = BuyerCardInfo.fromProto(b.buyer);
    return BlockedBuyerItem(
      buyerId: buyer.id,
      deleted: buyer.deleted,
      name: buyer.displayName.trim(),
      avatarUrl: buyer.avatarUrl.isEmpty ? null : buyer.avatarUrl,
      createTime: b.hasCreateTime() ? b.createTime.toDateTime() : null,
    );
  }

  final String buyerId;
  final bool deleted;
  final String name;
  final String? avatarUrl;

  /// 封鎖時間；清單依它新到舊排序，沒有時視為最舊。
  final DateTime? createTime;

  /// 清單的 keyset 順序：封鎖時間新到舊，相同時以買家 id 由大到小。小於 0 表示 this 排在 [other] 前面。
  int compareInList(BlockedBuyerItem other) {
    final a = createTime?.microsecondsSinceEpoch ?? 0;
    final b = other.createTime?.microsecondsSinceEpoch ?? 0;
    if (a != b) return b.compareTo(a);
    return other.buyerId.compareTo(buyerId);
  }

  /// 顯示名稱：已刪除的買家用目錄的 `buyer.Profile.deleted_name`，沒有名稱時用「買家」。
  String displayLabel(UiLocale l) => deleted
      ? buyerProfileDeletedName(l)
      : name.isNotEmpty
      ? name
      : '買家';
}

class BlockedBuyersPage {
  const BlockedBuyersPage({required this.items, required this.nextPageToken});

  final List<BlockedBuyerItem> items;
  final String nextPageToken;
}

/// 店家封鎖買家（`SellerAppService.BlockBuyer`、`UnblockBuyer`、`ListBlockedBuyers`）。
class StoreBlockRepository {
  StoreBlockRepository(this._seller);

  final SellerAppServiceClient _seller;

  /// 以貼文留言封鎖它的作者。
  Future<void> blockByComment(String commentId) async {
    await _seller.blockBuyer(pb.BlockBuyerRequest(commentId: commentId));
  }

  /// 以討論區留言封鎖它的作者。
  Future<void> blockByForumEntry(String forumEntryId) async {
    await _seller.blockBuyer(pb.BlockBuyerRequest(forumEntryId: forumEntryId));
  }

  /// 以聊天訊息封鎖它所屬對話的買家（呼叫者此刻的店要是該對話的店家）。
  Future<void> blockByMessage(String messageId) async {
    await _seller.blockBuyer(pb.BlockBuyerRequest(messageId: messageId));
  }

  /// 以評價封鎖寫這則評價的買家（要是呼叫者此刻的店的評價）。
  Future<void> blockByReview(String reviewId) async {
    await _seller.blockBuyer(pb.BlockBuyerRequest(reviewId: reviewId));
  }

  Future<void> unblock(String buyerId) async {
    await _seller.unblockBuyer(pb.UnblockBuyerRequest(buyerId: buyerId));
  }

  Future<BlockedBuyersPage> list({
    String pageToken = '',
    int pageSize = 20,
  }) async {
    final res = await _seller.listBlockedBuyers(
      pb.ListBlockedBuyersRequest(pageSize: pageSize, pageToken: pageToken),
    );
    return BlockedBuyersPage(
      items: res.blockedBuyers.map(BlockedBuyerItem.fromProto).toList(),
      nextPageToken: res.nextPageToken,
    );
  }
}
