import 'dart:async';

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../dashboard/data/dashboard_notification_unread_provider.dart';
import '../../order/data/after_sale_status_notifier.dart';
import '../../order/data/order_status_notifier.dart';
import '../../seller/data/seller_status_refresh.dart';
import '../presentation/notification_controller.dart';
import 'notification_repository.dart';

part 'state_refresh.g.dart';

/// 「該重讀了」的訊號對到的重讀動作。通知串流（`NotificationSignalService`）與前景推播的
/// `state_refresh_hints` 走同一份對照，不各寫一份。
class StateRefreshActions {
  const StateRefreshActions({
    required this.refreshInbox,
    required this.refreshOrders,
    required this.refreshAfterSales,
    required this.refreshSellerStatus,
  });

  final void Function() refreshInbox;
  final void Function() refreshOrders;
  final void Function() refreshAfterSales;
  final void Function() refreshSellerStatus;

  void applyHint(StateRefreshHint hint) {
    switch (hint) {
      case StateRefreshHint.STATE_REFRESH_HINT_ORDER:
        refreshOrders();
      case StateRefreshHint.STATE_REFRESH_HINT_AFTER_SALE:
        refreshAfterSales();
      case StateRefreshHint.STATE_REFRESH_HINT_SELLER_STATUS:
        refreshSellerStatus();
      default:
        break;
    }
  }

  void applyWorkspaceChange(SellerWorkspaceChange change) {
    switch (change) {
      case SellerWorkspaceChange.SELLER_WORKSPACE_CHANGE_ORDER:
        refreshOrders();
      case SellerWorkspaceChange.SELLER_WORKSPACE_CHANGE_AFTER_SALE:
        refreshAfterSales();
      default:
        break;
    }
  }

  /// 收到新的收件匣項目：重讀收件匣與未讀數，再依 [hints] 重讀對應資料。
  void applyInbox(Iterable<StateRefreshHint> hints) {
    refreshInbox();
    hints.forEach(applyHint);
  }

  /// 斷線重連、第一次連線：全部重讀。
  void applyAll() {
    refreshInbox();
    refreshOrders();
    refreshAfterSales();
    refreshSellerStatus();
  }
}

/// 推播 data 的 `state_refresh_hints`：以逗號分隔的 proto enum 名稱；不認得的名稱略過。
List<StateRefreshHint> parseStateRefreshHints(String? raw) {
  if (raw == null || raw.trim().isEmpty) return const [];
  final byName = {for (final h in StateRefreshHint.values) h.name: h};
  return [for (final name in raw.split(',')) ?byName[name.trim()]];
}

@Riverpod(keepAlive: true)
StateRefreshActions stateRefreshActions(Ref ref) => StateRefreshActions(
  refreshInbox: () {
    unawaited(
      ref.read(notificationListControllerProvider.notifier).silentRefresh(),
    );
    ref.invalidate(dashboardNotificationUnreadProvider);
  },
  refreshOrders: OrderStatusNotifier.instance.emitRefresh,
  refreshAfterSales: AfterSaleStatusNotifier.instance.emitRefreshAll,
  refreshSellerStatus: () => unawaited(
    ref.read(sellerStatusRefreshProvider.notifier).refresh(fresh: true),
  ),
);
