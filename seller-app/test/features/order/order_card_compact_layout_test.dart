import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/order/data/after_sale_model.dart';
import 'package:hawkr_seller/features/order/data/order_model.dart';
import 'package:hawkr_seller/features/order/data/seller_after_sale_lookup.dart';
import 'package:hawkr_seller/features/order/presentation/order_card.dart';
import 'package:hawkr_seller/features/order/presentation/order_card_widgets/order_bottom_row.dart';
import 'package:hawkr_seller/features/order/presentation/order_card_widgets/pill_button.dart';
import 'package:hawkr_seller/features/order/presentation/tcat_reservation_dialog.dart';
import 'package:hawkr_seller/features/shipment_anomaly/data/shipment_anomaly_providers.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  for (final width in [320.0, 402.0, 440.0]) {
    for (final scale in [1.0, 1.4]) {
      testWidgets('long order remains legible at $width / $scale text', (
        tester,
      ) async {
        const address = '臺北市中正區忠孝東路一段一百二十三號十二樓之五';
        const store = '臺北車站忠孝東路旗艦門市';
        final order = _order.copyWith(
          shippingMethod: '3002',
          pickupStoreName: store,
          shippingAddress: address,
          totalAmount: 123456789.99,
          items: const [
            OrderItem(name: '產地直送當季鮮採有機水果綜合禮盒特選大果', price: '125', quantity: 8),
          ],
        );
        await _pump(
          tester,
          width: width,
          scale: scale,
          child: OrderCard(
            order: order,
            isPending: true,
            isLoading: false,
            onSetLoading: (_, _) {},
            onReload: () {},
            onReloadAndSwitchTab: (_) {},
          ),
        );

        for (final text in [address, '7-11$store', r'NT$ 123456789.99']) {
          final paragraph = tester.renderObject<RenderParagraph>(
            find.text(text),
          );
          expect(paragraph.didExceedMaxLines, isFalse, reason: text);
          final rect = tester.getRect(find.text(text));
          expect(rect.left, greaterThanOrEqualTo(12));
          expect(rect.right, lessThanOrEqualTo(width - 12));
        }
        final amount = tester.widget<Text>(find.text(r'NT$ 123456789.99'));
        expect(amount.maxLines, isNull);
        expect(amount.overflow, isNull);
        expect(find.text('冷凍'), findsNothing);
        expect(find.text('共 1 件'), findsOneWidget);
        expect(find.text('預約取號'), findsOneWidget);
        expect(find.byTooltip('查看出貨說明'), findsOneWidget);
        final iconNames = tester
            .widgetList<HkIcon>(find.byType(HkIcon))
            .map((icon) => icon.name);
        expect(
          iconNames,
          containsAll([
            HkIconName.inventory,
            HkIconName.goods,
            HkIconName.copy,
            HkIconName.info,
            HkIconName.chevronDown,
          ]),
        );
        expect(
          iconNames.where((name) => name == HkIconName.chevronDown).length,
          2,
        );
        for (final button in tester.widgetList<PillButton>(
          find.byType(PillButton),
        )) {
          final rect = tester.getRect(find.byWidget(button));
          expect(rect.width, greaterThanOrEqualTo(48));
          expect(rect.height, greaterThanOrEqualTo(48));
        }
        expect(tester.takeException(), isNull);
      });
    }
  }

  testWidgets(
    'normal footer keeps copy and details inline and shipping actions compact',
    (tester) async {
      await _pump(tester, child: _footer(_order));
      final copy = tester.getRect(find.byTooltip('複製訂單編號'));
      final details = tester.getRect(find.widgetWithText(PillButton, '細節'));
      final reserve = tester.getRect(find.widgetWithText(PillButton, '預約出貨'));
      final info = tester.getRect(find.byTooltip('查看出貨說明'));
      expect(copy.center.dy, details.center.dy);
      expect(reserve.center.dy, info.center.dy);
      expect(reserve.width, lessThan(180));
      expect(tester.getSize(find.byType(OrderBottomRow)).height, 96);

      await tester.tap(find.widgetWithText(PillButton, '預約出貨'));
      await tester.pump();
      expect(find.byType(TcatReservationDialog), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets(
    'footer without shipping or after-sale actions occupies only one row',
    (tester) async {
      await _pump(tester, child: _footer(_order, pending: false));
      expect(tester.getSize(find.byType(OrderBottomRow)).height, 48);
      expect(find.text('細節'), findsOneWidget);
      expect(find.byTooltip('複製訂單編號'), findsOneWidget);
      expect(find.byTooltip('查看出貨說明'), findsNothing);
      expect(tester.takeException(), isNull);
    },
  );

  final scenarios = <(OrderSummary, String?)>[
    (_order, '預約出貨'),
    (_order.copyWith(labelUrl: 'https://fixture.invalid/label.pdf'), '呼叫黑貓取件'),
    (
      _order.copyWith(
        labelUrl: 'https://fixture.invalid/label.pdf',
        shipmentStatus: '210',
      ),
      '下載 PDF',
    ),
    (_order.copyWith(shippingMethod: '3002'), '預約取號'),
    (
      _order.copyWith(
        shippingMethod: '3002',
        odno: '12345678',
        validationNo: '1234',
      ),
      '出貨確認',
    ),
    (_order.copyWith(shippingMethod: '9999'), null),
  ];
  for (final scenario in scenarios) {
    testWidgets(
      'compact footer preserves ${scenario.$2 ?? 'unsupported shipping'} action',
      (tester) async {
        await _pump(
          tester,
          width: 320,
          scale: 1.4,
          child: _footer(scenario.$1),
        );
        if (scenario.$2 == null) {
          expect(find.text('此配送方式暫不支援 App 出貨'), findsOneWidget);
          expect(find.byTooltip('查看出貨說明'), findsNothing);
          expect(find.text('預約取號'), findsNothing);
          expect(find.text('預約出貨'), findsNothing);
        } else {
          expect(find.widgetWithText(PillButton, scenario.$2!), findsOneWidget);
          expect(find.byTooltip('查看出貨說明'), findsOneWidget);
        }
        expect(tester.takeException(), isNull);
      },
    );
  }

  for (final afterSale in const [
    (status: '0', label: '售後申請'),
    (status: '27', label: '確認收貨'),
  ]) {
    testWidgets('compact footer preserves ${afterSale.label}', (tester) async {
      await _pump(
        tester,
        width: 320,
        scale: 1.4,
        afterSale: AfterSaleCase(
          orderNo: _order.orderSn,
          status: afterSale.status,
        ),
        child: _footer(_order, pending: false),
      );
      expect(find.widgetWithText(PillButton, afterSale.label), findsOneWidget);
      expect(find.text('細節'), findsOneWidget);
      expect(tester.takeException(), isNull);
    });
  }
}

Widget _footer(OrderSummary order, {bool pending = true}) => OrderBottomRow(
  order: order,
  isPending: pending,
  onSetLoading: (_, _) {},
  onReload: () {},
  onReloadAndSwitchTab: (_) {},
);

Future<void> _pump(
  WidgetTester tester, {
  required Widget child,
  double width = 402,
  double scale = 1,
  AfterSaleCase? afterSale,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = Size(width, 874);
  addTearDown(tester.view.reset);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        sellerAfterSaleLookupProvider.overrideWith(
          () => _AfterSaleLookup(afterSale),
        ),
        shipmentAnomalyByOrderNoProvider.overrideWith((ref) async => {}),
      ],
      child: ScreenUtilInit(
        designSize: const Size(414, 896),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, _) => MaterialApp(
          theme: AppTheme.light,
          builder: (context, child) => MediaQuery(
            data: MediaQuery.of(
              context,
            ).copyWith(textScaler: TextScaler.linear(scale)),
            child: child!,
          ),
          home: Scaffold(
            body: SingleChildScrollView(
              padding: const EdgeInsets.all(12),
              child: child,
            ),
          ),
        ),
      ),
    ),
  );
  await tester.pump();
}

class _AfterSaleLookup extends SellerAfterSaleLookup {
  _AfterSaleLookup(this.afterSale);
  final AfterSaleCase? afterSale;
  @override
  Future<Map<String, AfterSaleCase>> build() async => {
    if (afterSale case final value?) value.orderNo: value,
  };
}

const _order = OrderSummary(
  id: 'O125369895',
  orderSn: 'O125369895',
  statusCode: 20,
  title: '訂單',
  subtitle: '20',
  dateTime: '2026-09-22T06:38:00+08:00',
  productName: '當季水果',
  price: '125',
  totalAmount: 250,
  buyerName: '陳小日',
  recipientName: '陳小日',
  shippingAddress: '桃園市桃園區鎮撫街1號',
  shippingMethod: '3001',
  shipmentStatus: '91',
  shippingStatus: '91',
  items: [OrderItem(name: '當季水果', price: '125', quantity: 2)],
);
