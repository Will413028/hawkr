import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../customer/data/customer_repository.dart';
import 'order_repository.dart';

part 'buyer_avatar_provider.g.dart';

/// 根據 buyerId 取得買家頭像 URL（帶快取）。
@Riverpod(keepAlive: true)
Future<String?> buyerAvatar(Ref ref, int buyerId) async {
  final repo = ref.watch(customerRepositoryProvider);
  final meta = await repo.getBuyerMeta(buyerId.toString());
  return meta?.avatarUrl;
}

/// 根據 orderNo 取得買家頭像 URL。
///
/// 流程：orderNo → order detail API (取 buyerId) → customer API (取 avatar)
/// keepAlive 確保同一訂單只查一次。
@Riverpod(keepAlive: true)
Future<String?> buyerAvatarByOrder(Ref ref, String orderNo) async {
  final orderRepo = ref.watch(orderRepositoryProvider);
  final detail = await orderRepo.fetchMatchedOrderDetails(orderSn: orderNo);
  final buyerId = detail.buyerId;
  if (buyerId == null) return null;

  // 複用 buyerAvatarProvider 的快取
  return ref.watch(buyerAvatarProvider(buyerId).future);
}
