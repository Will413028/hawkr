import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../shared/utils/hk_log.dart';
import '../../../seller/data/seller_status_refresh.dart';

/// 推播類型是認證或 PayUni 狀態變更：只當觸發訊號，狀態一律由伺服器重讀。
bool isSellerStatusPushType(String? type) {
  if (type == null || type.isEmpty) return false;
  final key = type.toUpperCase();
  return key == 'PAYUNI_STATUS' ||
      key == 'SELLER_REVIEW_STATUS' ||
      key == 'MERCHANT_VERIFICATION' ||
      key.startsWith('CERTIFICATION_') ||
      key.startsWith('MERCHANT.VERIFICATION.');
}

/// 伺服器剛改變了認證或 PayUni 狀態（推播）：重讀店家、認證流程與 PayUni 註冊。每個通知事件只呼叫一次，用 manager 的 container；
/// [container] 是 null（manager 還沒注入）時略過。
void refreshSellerStatusFromPush(ProviderContainer? container) {
  if (container == null) {
    HKLog.logD('[SellerStatusPush] 取不到 container，略過重讀');
    return;
  }
  unawaited(
    container.read(sellerStatusRefreshProvider.notifier).refresh(fresh: true),
  );
}
