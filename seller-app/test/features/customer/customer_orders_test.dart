import 'package:hawkr_seller/l10n/l10n.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/crm/v1alpha1/crm.pb.dart' as crm;
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_seller/features/customer/data/customer_model.dart';
import 'package:hawkr_seller/features/customer/data/customer_orders.dart';
import 'package:hawkr_seller/features/customer/presentation/customer_controller.dart';
import 'package:hawkr_seller/features/customer/presentation/customer_orders_screen.dart';
import 'package:hawkr_seller/features/order/data/order_model.dart';
import 'package:hawkr_seller/features/order/data/order_repository.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

OrderSummary _order(int? status) => OrderSummary(
  id: 'id',
  orderSn: 'SN1',
  statusCode: status,
  title: '',
  subtitle: '',
  dateTime: '2026/11/01',
  productName: '蘋果',
  price: '100',
  totalAmount: 100.4,
);

class _FakeOrders implements OrderRepository {
  _FakeOrders(this.pages);

  /// 第 N 頁（從 1 起）的回應；超出就是空頁。
  final List<Object> pages;
  final calls = <({int page, int pageSize, String? buyerId})>[];

  @override
  Future<List<OrderSummary>> fetchMatchedOrders({
    int page = 1,
    int pageSize = 10,
    String? status,
    String? buyerId,
  }) async {
    calls.add((page: page, pageSize: pageSize, buyerId: buyerId));
    if (page > pages.length) return const [];
    final r = pages[page - 1];
    if (r is Exception) throw r;
    return r as List<OrderSummary>;
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

List<OrderSummary> _many(int n, {String prefix = 'o'}) => [
  for (var i = 0; i < n; i++)
    OrderSummary(
      id: '$prefix$i',
      orderSn: '$prefix$i',
      statusCode: 50,
      title: '',
      subtitle: '',
      dateTime: '2026/11/01',
      productName: '蘋果',
      price: '100',
      totalAmount: 100,
    ),
];

Future<void> _pump(WidgetTester tester, _FakeOrders repo) async {
  tester.view.physicalSize = const Size(390, 844);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [orderRepositoryProvider.overrideWithValue(repo)],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        builder: (_, _) => const MaterialApp(
          locale: Locale('zh'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,

          home: CustomerOrdersScreen(customerId: '42', customerName: '甲'),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  test('order summary maps to a record by status code', () {
    expect(
      orderRecordFromSummary(_order(0)).status,
      OrderStatus.pendingPayment,
    );
    expect(orderRecordFromSummary(_order(35)).status, OrderStatus.issue);
    expect(orderRecordFromSummary(_order(50)).status, OrderStatus.completed);
    expect(orderRecordFromSummary(_order(54)).status, OrderStatus.completed);
    expect(orderRecordFromSummary(_order(70)).status, OrderStatus.completed);
    expect(orderRecordFromSummary(_order(80)).status, OrderStatus.cancelled);
    for (final c in [10, 20, 30, 32, 40]) {
      expect(orderRecordFromSummary(_order(c)).status, OrderStatus.delivering);
    }
    final r = orderRecordFromSummary(_order(50));
    expect([r.orderId, r.items, r.amount], ['SN1', '蘋果', 100]);
  });

  test('the id used to filter orders is the CRM buyer_id', () {
    final c = Customer.fromProto(crm.Customer(buyerId: 'buyer-9'));
    expect(c.id, 'buyer-9');
  });

  testWidgets('no orders shows the empty message only', (tester) async {
    await _pump(tester, _FakeOrders([<OrderSummary>[]]));
    expect(find.text('暫無訂單'), findsOneWidget);
    expect(find.byKey(const ValueKey('customer-orders-error')), findsNothing);
  });

  testWidgets('a read failure is not shown as no orders', (tester) async {
    await _pump(tester, _FakeOrders([ConnectException(Code.unavailable, 'x')]));
    expect(find.byKey(const ValueKey('customer-orders-error')), findsOneWidget);
    expect(find.text('暫無訂單'), findsNothing);
  });

  test('orders page past 50 with the buyer id until a short page', () async {
    final repo = _FakeOrders([
      _many(customerOrdersPageSize),
      _many(3, prefix: 'p'),
    ]);
    final c = ProviderContainer(
      overrides: [orderRepositoryProvider.overrideWithValue(repo)],
    );
    addTearDown(c.dispose);
    c.listen(customerOrdersProvider('42'), (_, _) {});
    var s = await c.read(customerOrdersProvider('42').future);
    expect(s.orders, hasLength(50));
    expect(s.hasMore, isTrue);
    await c.read(customerOrdersProvider('42').notifier).loadMore();
    s = c.read(customerOrdersProvider('42')).value!;
    expect(s.orders, hasLength(53));
    expect(s.hasMore, isFalse);
    expect(repo.calls.map((e) => (e.page, e.pageSize, e.buyerId)), [
      (1, 50, '42'),
      (2, 50, '42'),
    ]);
  });

  test('a failed next page keeps the loaded orders and can retry', () async {
    final repo = _FakeOrders([
      _many(customerOrdersPageSize),
      ConnectException(Code.unavailable, 'x'),
    ]);
    final c = ProviderContainer(
      overrides: [orderRepositoryProvider.overrideWithValue(repo)],
    );
    addTearDown(c.dispose);
    c.listen(customerOrdersProvider('42'), (_, _) {});
    await c.read(customerOrdersProvider('42').future);
    await c.read(customerOrdersProvider('42').notifier).loadMore();
    final s = c.read(customerOrdersProvider('42')).value!;
    expect(s.orders, hasLength(50));
    expect(s.loadMoreError, isA<ConnectException>());
    expect(s.loadingMore, isFalse);
    expect(s.hasMore, isTrue);
  });

  testWidgets('the footer loads the next page and the list shows it', (
    tester,
  ) async {
    final repo = _FakeOrders([
      _many(customerOrdersPageSize),
      _many(2, prefix: 'p'),
    ]);
    await _pump(tester, repo);
    await tester.drag(find.byType(ListView), const Offset(0, -100000));
    await tester.pumpAndSettle();
    expect(repo.calls.map((e) => e.page), [1, 2]);
  });
}
