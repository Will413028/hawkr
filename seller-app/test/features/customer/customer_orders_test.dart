import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_seller/features/customer/data/customer_model.dart';
import 'package:hawkr_seller/features/customer/data/customer_orders.dart';
import 'package:hawkr_seller/features/customer/presentation/customer_controller.dart';
import 'package:hawkr_seller/features/customer/presentation/customer_orders_screen.dart';
import 'package:hawkr_seller/features/order/data/order_model.dart';
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

Future<void> _pump(
  WidgetTester tester,
  Future<List<OrderRecord>> Function() f,
) async {
  tester.view.physicalSize = const Size(390, 844);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [customerOrdersProvider('42').overrideWith((ref) => f())],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        builder: (_, _) => const MaterialApp(
          home: CustomerOrdersScreen(customerId: '42', customerName: '甲'),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  test('order summary maps to a record by status code', () {
    expect(orderRecordFromSummary(_order(50)).status, OrderStatus.completed);
    expect(orderRecordFromSummary(_order(70)).status, OrderStatus.completed);
    expect(orderRecordFromSummary(_order(80)).status, OrderStatus.cancelled);
    expect(orderRecordFromSummary(_order(20)).status, OrderStatus.delivering);
    final r = orderRecordFromSummary(_order(50));
    expect([r.orderId, r.items, r.amount], ['SN1', '蘋果', 100]);
  });

  testWidgets('no orders shows the empty message only', (tester) async {
    await _pump(tester, () async => const []);
    expect(find.text('暫無訂單'), findsOneWidget);
    expect(find.byKey(const ValueKey('customer-orders-error')), findsNothing);
  });

  testWidgets('a read failure is not shown as no orders', (tester) async {
    await _pump(
      tester,
      () async => throw ConnectException(Code.unavailable, 'x'),
    );
    expect(find.byKey(const ValueKey('customer-orders-error')), findsOneWidget);
    expect(find.text('暫無訂單'), findsNothing);
  });
}
