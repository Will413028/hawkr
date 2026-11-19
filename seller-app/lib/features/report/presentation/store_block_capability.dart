import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart'
    show SellerCapability;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../seller/data/current_store.dart';

part 'store_block_capability.g.dart';

/// 店家有封鎖買家的能力（`SELLER_CAPABILITY_CONTENT_CONTRACT`）。店家還沒讀到時不下結論（true），伺服器是最後一道。
@riverpod
bool canBlockBuyers(Ref ref) =>
    ref
        .watch(myStoreSnapshotProvider)
        ?.can(SellerCapability.SELLER_CAPABILITY_CONTENT_CONTRACT) ??
    true;
