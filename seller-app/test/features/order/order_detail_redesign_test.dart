import 'package:hawkr_api/gen/hawkr/payment/v1alpha1/payment.pb.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_seller/features/order/data/order_model.dart';
import 'package:hawkr_seller/features/wallet/data/earnings.dart';
import 'package:hawkr_seller/features/order/presentation/shared_order_detail_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_seller/l10n/gen/app_localizations.dart';

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

  testWidgets('入帳紀錄列出這張訂單的入帳列與淨入帳', (tester) async {
    final earnings = GetMyOrderEarningsResponse(
      entries: [
        EarningsEntry(
          id: 'r1',
          orderNo: 'O-1',
          type: EarningsEntryType.EARNINGS_ENTRY_TYPE_REVERSAL,
          status: EarningsEntryStatus.EARNINGS_ENTRY_STATUS_COMPLETED,
        )..mergeFromProto3Json({'amount': '-20'}),
        EarningsEntry(
          id: 't1',
          orderNo: 'O-1',
          type: EarningsEntryType.EARNINGS_ENTRY_TYPE_TRANSFER,
          status: EarningsEntryStatus.EARNINGS_ENTRY_STATUS_COMPLETED,
        )..mergeFromProto3Json({'amount': '160'}),
      ],
    )..mergeFromProto3Json({'netAmount': '140'});
    await tester.pumpWidget(
      _TestApp(
        order: _detailedOrder,
        orderId: 'O-1',
        overrides: [
          myOrderEarningsProvider('O-1').overrideWith((_) async => earnings),
        ],
      ),
    );
    await tester.pump();
    await tester.dragUntilVisible(
      find.byKey(const ValueKey('order-earnings-net')),
      find.byType(ListView).first,
      const Offset(0, -260),
    );

    expect(find.text('入帳紀錄'), findsOneWidget);
    expect(find.text('扣回'), findsOneWidget);
    expect(find.text('入帳'), findsOneWidget);
    expect(find.text('NT\$ -20'), findsOneWidget);
    expect(find.text('淨入帳 NT\$ 140'), findsOneWidget);
  });

  testWidgets('訂單沒有付款（not_found）時不顯示入帳紀錄', (tester) async {
    await tester.pumpWidget(
      _TestApp(
        order: _detailedOrder,
        orderId: 'O-1',
        overrides: [
          myOrderEarningsProvider('O-1').overrideWith((_) async => null),
        ],
      ),
    );
    await tester.pump();
    expect(find.text('入帳紀錄'), findsNothing);
  });
}

class _TestApp extends StatelessWidget {
  const _TestApp({
    required this.order,
    this.orderId = '',
    this.overrides = const [],
  });

  final OrderSummary order;
  final String orderId;
  final List<Override> overrides;

  @override
  Widget build(BuildContext context) {
    return ProviderScope(
      overrides: overrides,
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        builder: (_, _) => MaterialApp(
          locale: const Locale('zh'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: SharedOrderDetailScreen(
            orderId: orderId,
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
  shippingMethod: '3001',
  shippingAddress: '桃園市桃園區鎮撫街1號',
  phone: '0912345678',
  items: [
    OrderItem(name: '蟹味棒', price: '100', actualPayAmount: '100', quantity: 1),
  ],
);
