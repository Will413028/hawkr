import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pbenum.dart';
import 'package:hawkr_buyer/features/notification/data/push/destination.dart';
import 'package:hawkr_buyer/features/notification/data/state_refresh.dart';
import 'package:hawkr_buyer/features/order/data/after_sale_status_notifier.dart';
import 'package:hawkr_buyer/features/order/data/order_status_notifier.dart';
import 'package:hawkr_buyer/features/order/data/refresh_coalescer.dart';

void main() {
  const afterSale = [StateRefreshHint.STATE_REFRESH_HINT_AFTER_SALE];
  const order = [StateRefreshHint.STATE_REFRESH_HINT_ORDER];

  testWidgets('同一事件的串流訊號與前景推播提示只送一次重讀', (tester) async {
    final afterSales = <AfterSaleStatusEvent>[];
    final orders = <OrderStatusEvent>[];
    final a = AfterSaleStatusNotifier.instance.eventStream.listen(
      afterSales.add,
    );
    final b = OrderStatusNotifier.instance.eventStream.listen(orders.add);

    dispatchStateRefreshHints(afterSale + order);
    await tester.pump(const Duration(milliseconds: 50));
    dispatchStateRefreshHints(
      stateRefreshHintsFromPush({
        notificationStateRefreshHintsKey:
            'STATE_REFRESH_HINT_AFTER_SALE,STATE_REFRESH_HINT_ORDER',
      }),
    );
    await tester.pump(RefreshCoalescer.defaultWindow);

    expect((afterSales.length, orders.length), (1, 1));

    dispatchStateRefreshHints(afterSale);
    await tester.pump(RefreshCoalescer.defaultWindow);
    expect((afterSales.length, orders.length), (2, 1));

    a.cancel();
    b.cancel();
  });
}
