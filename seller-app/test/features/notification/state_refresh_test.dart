import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_seller/features/notification/data/notification_repository.dart';
import 'package:hawkr_seller/features/notification/data/state_refresh.dart';

void main() {
  late List<String> calls;
  late StateRefreshActions actions;

  setUp(() {
    calls = [];
    actions = StateRefreshActions(
      refreshInbox: () => calls.add('inbox'),
      refreshOrders: () => calls.add('orders'),
      refreshAfterSales: () => calls.add('afterSales'),
      refreshSellerStatus: () => calls.add('sellerStatus'),
    );
  });

  test('each hint maps to its refresh; unspecified does nothing', () {
    for (final h in StateRefreshHint.values) {
      actions.applyHint(h);
    }
    expect(calls, ['orders', 'afterSales', 'sellerStatus']);
  });

  test('inbox signal refreshes the inbox first, then the hinted data', () {
    actions.applyInbox([
      StateRefreshHint.STATE_REFRESH_HINT_AFTER_SALE,
      StateRefreshHint.STATE_REFRESH_HINT_ORDER,
    ]);
    expect(calls, ['inbox', 'afterSales', 'orders']);
  });

  test('workspace events map to order and after-sale refresh', () {
    actions
      ..applyWorkspaceChange(
        SellerWorkspaceChange.SELLER_WORKSPACE_CHANGE_ORDER,
      )
      ..applyWorkspaceChange(
        SellerWorkspaceChange.SELLER_WORKSPACE_CHANGE_AFTER_SALE,
      )
      ..applyWorkspaceChange(
        SellerWorkspaceChange.SELLER_WORKSPACE_CHANGE_UNSPECIFIED,
      );
    expect(calls, ['orders', 'afterSales']);
  });

  test('applyAll refreshes everything', () {
    actions.applyAll();
    expect(calls, ['inbox', 'orders', 'afterSales', 'sellerStatus']);
  });

  test('push data hints parse by proto enum name and skip unknown names', () {
    expect(parseStateRefreshHints(null), isEmpty);
    expect(parseStateRefreshHints(''), isEmpty);
    expect(
      parseStateRefreshHints(
        'STATE_REFRESH_HINT_ORDER, BOGUS,STATE_REFRESH_HINT_SELLER_STATUS',
      ),
      [
        StateRefreshHint.STATE_REFRESH_HINT_ORDER,
        StateRefreshHint.STATE_REFRESH_HINT_SELLER_STATUS,
      ],
    );
  });
}
