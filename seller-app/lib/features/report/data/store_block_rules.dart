import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../seller/data/current_store.dart';

/// 店家有封鎖買家的能力（`SELLER_CAPABILITY_CONTENT_CONTRACT`）；沒讀到店家時是 false。
final canBlockBuyersProvider = Provider<bool>(
  (ref) =>
      ref
          .watch(myStoreSnapshotProvider)
          ?.can(SellerCapability.SELLER_CAPABILITY_CONTENT_CONTRACT) ??
      false,
);

/// 封鎖作者的入口顯示規則（貼文留言與討論區共用）：店家有封鎖能力、不是自己寫的、
/// 作者是買家、而且伺服器允許觀看者刪（代表它在這家店的內容裡）。
bool canBlockAuthor({
  required bool canBlockBuyers,
  required bool isMine,
  required bool viewerCanDelete,
  required bool authorIsBuyer,
}) => canBlockBuyers && authorIsBuyer && !isMine && viewerCanDelete;

/// 找不到作者名稱時，封鎖彈窗與 toast 共用的稱呼。
String blockTargetName(String name) => name.trim().isEmpty ? '此人' : name;
