import 'package:hawkr_seller/features/order/data/order_model.dart';
import 'package:hawkr_seller/features/order/presentation/shared_order_detail_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  const statusLabels = <int, String>{
    0: '待付款',
    10: '已付款',
    20: '待出貨',
    30: '配送中',
    32: '待取貨',
    35: '物流異常',
    40: '已送達',
    50: '已完成',
    60: '部分退款',
    70: '全額退款',
    80: '已取消',
  };

  for (final entry in statusLabels.entries) {
    testWidgets('新版訂單細節顯示 ${entry.value} 狀態', (tester) async {
      await tester.pumpWidget(_TestApp(order: _order(statusCode: entry.key)));
      await tester.pump();

      expect(find.text('訂單細節'), findsOneWidget);
      expect(find.text('交易總覽'), findsOneWidget);
      expect(find.text(entry.value), findsOneWidget);
    });
  }

  testWidgets('order detail shows no delivery temperature row', (tester) async {
    await tester.pumpWidget(const _TestApp(order: _detailedOrder));
    await tester.pump();
    await tester.dragUntilVisible(
      find.text('配送方式'),
      find.byType(ListView).first,
      const Offset(0, -200),
    );
    expect(find.text('配送溫層'), findsNothing);
  });

  testWidgets('買家卡、訂單資訊與新操作文案保持在同一頁', (tester) async {
    await tester.pumpWidget(const _TestApp(order: _detailedOrder));
    await tester.pump();

    expect(find.text('陳小日'), findsOneWidget);
    expect(find.text('聯絡買家'), findsOneWidget);
    expect(find.text('訂單編號'), findsOneWidget);
    expect(find.text('配送地址'), findsOneWidget);

    await tester.dragUntilVisible(
      find.text('蟹味棒'),
      find.byType(ListView).first,
      const Offset(0, -260),
    );
    expect(find.text('商品明細'), findsOneWidget);
    expect(find.text('蟹味棒'), findsOneWidget);

    await tester.dragUntilVisible(
      find.text('折扣券'),
      find.byType(ListView).first,
      const Offset(0, -260),
    );
    expect(find.text('付款詳情'), findsOneWidget);
    expect(find.text('折扣券'), findsOneWidget);
  });
}

class _TestApp extends StatelessWidget {
  const _TestApp({required this.order});

  final OrderSummary order;

  @override
  Widget build(BuildContext context) {
    return ProviderScope(
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        builder: (_, __) => MaterialApp(
          home: SharedOrderDetailScreen(
            // 空 id 可避免 widget test 觸發後端；生產 route 會傳真實單號。
            orderId: '',
            initialSummary: order,
            fallbackDateText: order.dateTime,
            fallbackItemsText: order.productName,
            fallbackAmount: order.totalAmount.round(),
          ),
        ),
      ),
    );
  }
}

OrderSummary _order({required int statusCode}) => OrderSummary(
  id: '',
  statusCode: statusCode,
  title: '',
  subtitle: statusCode.toString(),
  dateTime: '2026-07-16T06:38:00+08:00',
  productName: '蟹味棒',
  price: '100',
  totalAmount: 100,
  buyerName: '陳小日',
  recipientName: '陳小日',
  items: const [OrderItem(name: '蟹味棒', price: '100')],
);

const _detailedOrder = OrderSummary(
  id: '',
  statusCode: 20,
  title: '',
  subtitle: '20',
  dateTime: '2026-07-16T06:38:00+08:00',
  productName: '蟹味棒',
  price: '100',
  totalAmount: 160,
  shippingFee: 60,
  discountAmount: 10,
  buyerId: 123,
  buyerName: '陳小日',
  recipientName: '陳小日',
  // 真實賣家訂單一定帶 CodeMeta ShippingMethod value（order-service 建單時寫入）；
  // 留空會走「未知配送方式」的 fail-safe 呈現（label 變成中性的「配送資訊」）。
  shippingMethod: '3001',
  shippingAddress: '桃園市桃園區鎮撫街1號',
  phone: '0912345678',
  items: [
    OrderItem(name: '蟹味棒', price: '100', actualPayAmount: '100', quantity: 1),
  ],
);
