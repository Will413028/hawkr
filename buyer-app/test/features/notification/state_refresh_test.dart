import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pbenum.dart';
import 'package:hawkr_buyer/features/notification/data/push/destination.dart';
import 'package:hawkr_buyer/features/notification/data/state_refresh.dart';
import 'package:hawkr_buyer/features/order/data/after_sale_status_notifier.dart';
import 'package:hawkr_buyer/features/order/data/order_status_notifier.dart';
import 'package:hawkr_buyer/features/order/data/refresh_coalescer.dart';

void main() {
  setUp(() {
    OrderStatusNotifier.instance.coalesceWindow = Duration.zero;
    AfterSaleStatusNotifier.instance.coalesceWindow = Duration.zero;
    addTearDown(() {
      OrderStatusNotifier.instance.coalesceWindow =
          RefreshCoalescer.defaultWindow;
      AfterSaleStatusNotifier.instance.coalesceWindow =
          RefreshCoalescer.defaultWindow;
    });
  });
  test('前景推播的 state_refresh_hints 解析：逗號分隔、略過不認得與 UNSPECIFIED', () {
    expect(
      stateRefreshHintsFromPush({
        notificationStateRefreshHintsKey:
            'STATE_REFRESH_HINT_ORDER, STATE_REFRESH_HINT_AFTER_SALE,STATE_REFRESH_HINT_ORDER,'
            'STATE_REFRESH_HINT_UNSPECIFIED,bogus,',
      }),
      [
        StateRefreshHint.STATE_REFRESH_HINT_ORDER,
        StateRefreshHint.STATE_REFRESH_HINT_AFTER_SALE,
      ],
    );
    expect(stateRefreshHintsFromPush(const {}), isEmpty);
    expect(
      stateRefreshHintsFromPush({notificationStateRefreshHintsKey: ''}),
      isEmpty,
    );
  });

  test('前景推播的提示 → 訂單、售後通知器重讀', () async {
    final orders = <OrderStatusEvent>[];
    final afterSales = <AfterSaleStatusEvent>[];
    final a = OrderStatusNotifier.instance.eventStream.listen(orders.add);
    final b = AfterSaleStatusNotifier.instance.eventStream.listen(
      afterSales.add,
    );
    addTearDown(a.cancel);
    addTearDown(b.cancel);

    dispatchStateRefreshHints(
      stateRefreshHintsFromPush({
        notificationStateRefreshHintsKey: 'STATE_REFRESH_HINT_ORDER',
      }),
    );
    await pumpEventQueue();
    expect((orders.length, afterSales.length), (1, 0));

    dispatchStateRefreshHints(
      stateRefreshHintsFromPush({
        notificationStateRefreshHintsKey:
            'STATE_REFRESH_HINT_AFTER_SALE,STATE_REFRESH_HINT_SELLER_STATUS',
      }),
    );
    await pumpEventQueue();
    expect((orders.length, afterSales.length), (1, 1));
  });

  test('點推播：有 inbox_item_id 才標已讀，失敗不拋', () async {
    final marked = <String>[];
    await markTappedInboxItemRead({
      notificationInboxItemIdKey: ' item-7 ',
    }, (id) async => marked.add(id));
    await markTappedInboxItemRead(const {}, (id) async => marked.add(id));
    await markTappedInboxItemRead({
      notificationInboxItemIdKey: '',
    }, (id) async => marked.add(id));
    expect(marked, ['item-7']);

    await markTappedInboxItemRead({
      notificationInboxItemIdKey: 'item-8',
    }, (_) async => throw StateError('offline'));
  });
}
