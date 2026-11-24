import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pbenum.dart';

import '../../order/data/after_sale_status_notifier.dart';
import '../../order/data/order_status_notifier.dart';
import 'push/destination.dart';

/// 推播 `data['state_refresh_hints']`（逗號分隔的 proto enum 名稱）解析成提示；空、不認得與 UNSPECIFIED 略過。
List<StateRefreshHint> stateRefreshHintsFromPush(Map<String, dynamic> data) {
  final raw = data[notificationStateRefreshHintsKey]?.toString() ?? '';
  final names = raw.split(',').map((s) => s.trim()).where((s) => s.isNotEmpty);
  final hints = <StateRefreshHint>{};
  for (final name in names) {
    for (final hint in StateRefreshHint.values) {
      if (hint.name == name &&
          hint != StateRefreshHint.STATE_REFRESH_HINT_UNSPECIFIED) {
        hints.add(hint);
      }
    }
  }
  return hints.toList(growable: false);
}

/// 依提示叫對應的狀態通知器重讀。買家 App 沒有賣家狀態，`SELLER_STATUS` 不處理。
void dispatchStateRefreshHints(Iterable<StateRefreshHint> hints) {
  for (final hint in hints.toSet()) {
    switch (hint) {
      case StateRefreshHint.STATE_REFRESH_HINT_ORDER:
        OrderStatusNotifier.instance.requestRefresh();
      case StateRefreshHint.STATE_REFRESH_HINT_AFTER_SALE:
        AfterSaleStatusNotifier.instance.requestRefresh();
      default:
        break;
    }
  }
}

/// 串流重連後：訊號可能遺漏，全部狀態重讀。
void dispatchFullStateRefresh() => dispatchStateRefreshHints(const [
  StateRefreshHint.STATE_REFRESH_HINT_ORDER,
  StateRefreshHint.STATE_REFRESH_HINT_AFTER_SALE,
]);

/// 點推播時 `inbox_item_id` 有值就標已讀（背景呼叫，失敗不影響導覽）。
Future<void> markTappedInboxItemRead(
  Map<String, dynamic> data,
  Future<void> Function(String id) markRead,
) async {
  final id = data[notificationInboxItemIdKey]?.toString().trim() ?? '';
  if (id.isEmpty) return;
  try {
    await markRead(id);
  } catch (_) {}
}
